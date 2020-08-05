package logclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/apex/log"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"

	"github.com/kafkaesque-io/burnell/src/logstream"
	"github.com/kafkaesque-io/burnell/src/pb"
	"github.com/kafkaesque-io/burnell/src/util"
)

// FunctionLogResponse is HTTP response object
type FunctionLogResponse struct {
	Logs             string
	BackwardPosition int64
	ForwardPosition  int64
}

// ErrNotFoundFunction error for function not found
var ErrNotFoundFunction = fmt.Errorf("function not found")

var logger = log.WithFields(log.Fields{"app": "pulsar-function-listener"})

var functionWorkerDomain = os.Getenv("FunctionWorkerDomain")

// FunctionLogRequest is HTTP resquest object
type FunctionLogRequest struct {
	Bytes            int64  `json:"bytes"`
	BackwardPosition int64  `json:"backwardPosition"`
	ForwardPosition  int64  `json:"forwardPosition"`
	Direction        string `json:"direction"`
}

// FunctionType is the object encapsulates all the function attributes
type FunctionType struct {
	Tenant           string
	Namespace        string
	FunctionName     string
	FunctionWorkerID string
	InputTopics      []string
	InputTopicRegex  string
	SinkTopic        string
	LogTopic         string
	AutoAck          bool
	Parallism        int32
}

// the signal to track if the liveness of the reader process
type liveSignal struct{}

// functionMap stores FunctionType object and the key is tenant+namespace+function name
var functionMap = make(map[string]FunctionType)
var fnMpLock = sync.RWMutex{}

// ReadFunctionMap reads a thread safe map
func ReadFunctionMap(key string) (FunctionType, bool) {
	fnMpLock.RLock()
	defer fnMpLock.RUnlock()
	f, ok := functionMap[key]
	return f, ok
}

// WriteFunctionMap writes a key/value to a thread safe map
func WriteFunctionMap(key string, f FunctionType) {
	fnMpLock.Lock()
	defer fnMpLock.Unlock()
	functionMap[key] = f
}

// DeleteFunctionMap deletes a key from a thread safe map
func DeleteFunctionMap(key string) bool {
	fnMpLock.Lock()
	defer fnMpLock.Unlock()
	if _, ok := functionMap[key]; ok {
		delete(functionMap, key)
		return ok
	}
	return false
}

// TenantFunctionCount returns the number of functions under the tenant
func TenantFunctionCount(tenant string) int {
	counter := 0
	for _, v := range functionMap {
		if v.Tenant == tenant {
			counter++
		}

	}
	return counter
}

// ReaderLoop continuously reads messages from function metadata topic
func ReaderLoop(sig chan *liveSignal) {
	defer func(s chan *liveSignal) {
		logger.Errorf("function listener terminated")
		s <- &liveSignal{}
	}(sig)

	// Configuration variables pertaining to this reader
	tokenStr := util.GetConfig().PulsarToken
	uri := util.GetConfig().PulsarURL
	topicName := "persistent://public/functions/metadata"

	clientOpt := pulsar.ClientOptions{
		URL:               uri,
		OperationTimeout:  30 * time.Second,
		ConnectionTimeout: 30 * time.Second,
	}

	if tokenStr != "" {
		clientOpt.Authentication = pulsar.NewAuthenticationToken(tokenStr)
	}

	if strings.HasPrefix(uri, "pulsar+ssl://") {
		trustStore := util.AssignString(util.GetConfig().TrustStore, "/etc/ssl/certs/ca-bundle.crt")
		clientOpt.TLSTrustCertsFilePath = trustStore
	}
	// Pulsar client
	client, err := pulsar.NewClient(clientOpt)
	if err != nil {
		logger.Errorf("pulsar.NewClient %v", err)
		return
	}

	defer client.Close()

	reader, err := client.CreateReader(pulsar.ReaderOptions{
		Topic:          topicName,
		StartMessageID: pulsar.EarliestMessageID(),
	})

	if err != nil {
		logger.Errorf("pulsar.CreateReader %v", err)
		return
	}

	defer reader.Close()

	ctx := context.Background()

	// infinite loop to receive messages
	for {
		msg, err := reader.Next(ctx)
		if err != nil {
			logger.Errorf("pulsar.reader.Next %v", err)
			return
		}
		sr := pb.ServiceRequest{}
		proto.Unmarshal(msg.Payload(), &sr)
		ParseServiceRequest(sr.GetFunctionMetaData(), sr.GetWorkerId(), sr.GetServiceRequestType())
		//logger.Infof("total number of tenants %d, the total number of functions %d", len(tenantFunctionCounter), len(functionMap))
	}
}

// ParseServiceRequest build a Function object based on Pulsar function metadata message
func ParseServiceRequest(sr *pb.FunctionMetaData, workerID string, serviceType pb.ServiceRequest_ServiceRequestType) {
	fd := sr.FunctionDetails
	key := fd.GetTenant() + fd.GetNamespace() + fd.GetName()
	if serviceType == pb.ServiceRequest_DELETE {
		DeleteFunctionMap(key)
	} else {
		f := FunctionType{
			Tenant:           fd.GetTenant(),
			Namespace:        fd.GetNamespace(),
			FunctionName:     fd.GetName(),
			FunctionWorkerID: workerID,
			InputTopicRegex:  fd.Source.GetTopicsPattern(),
			SinkTopic:        fd.Sink.Topic,
			LogTopic:         fd.GetLogTopic(),
			AutoAck:          fd.GetAutoAck(),
			Parallism:        fd.GetParallelism(),
		}
		for k := range fd.Source.InputSpecs {
			f.InputTopics = append(f.InputTopics, k)
		}
		if len(fd.Source.TopicsPattern) > 0 {
			f.InputTopics = append(f.InputTopics, fd.Source.TopicsPattern)
		}
		WriteFunctionMap(key, f)
	}
}

// FunctionTopicWatchDog is a watch dog for the function topic reader process
func FunctionTopicWatchDog() {

	go func() {
		s := make(chan *liveSignal)
		go ReaderLoop(s)
		for {
			select {
			case <-s:
				go ReaderLoop(s)
			}
		}
	}()
}

// GetFunctionLog gets the logs from the function worker process
// Since the function may get reassigned after restart, we will establish the connection every time the log request is being made.
func GetFunctionLog(functionName, instance string, rd FunctionLogRequest) (FunctionLogResponse, error) {
	// var funcWorker string
	function, ok := ReadFunctionMap(functionName)
	if !ok {
		return FunctionLogResponse{}, ErrNotFoundFunction
	}
	// Set up a connection to the server.
	fqdn := function.FunctionWorkerID + functionWorkerDomain
	address := fqdn + util.AssignString(util.GetConfig().LogServerPort, logstream.DefaultLogServerPort)
	// address = logstream.DefaultLogServerPort
	logger.Infof("connect to function worker address %s", address)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(600*time.Second))
	if err != nil {
		logger.Errorf("grpc.Dial to log server error %v", err)
		return FunctionLogResponse{}, err
	}
	defer conn.Close()
	c := logstream.NewLogStreamClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	direction := requestDirection(rd)
	req := &logstream.ReadRequest{
		File:          logstream.FunctionLogPath(function.Tenant, function.Namespace, function.FunctionName, instance),
		Direction:     direction,
		Bytes:         rd.Bytes,
		ForwardIndex:  rd.ForwardPosition,
		BackwardIndex: rd.BackwardPosition,
	}
	logger.Debugf("making a remote call %v", req)
	res, err := c.Read(ctx, req)
	if err != nil {
		logger.Errorf("grcp call to log server failed : %v", err)
		return FunctionLogResponse{}, err
	}
	logger.Debugf("logs: %s %v %v", res.GetLogs(), res.GetBackwardIndex(), res.GetForwardIndex())
	return FunctionLogResponse{
		Logs:             res.GetLogs(),
		BackwardPosition: res.GetBackwardIndex(),
		ForwardPosition:  res.GetForwardIndex(),
	}, nil
}

func requestDirection(rd FunctionLogRequest) logstream.ReadRequest_Direction {
	if rd.ForwardPosition > 0 {
		return logstream.ReadRequest_FORWARD
	}
	return logstream.ReadRequest_BACKWARD
}

// /pulsar/logs/functions/ming-luo/namespace2/for-monitor-function
