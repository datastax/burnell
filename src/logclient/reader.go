package logclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
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

// InstanceStatus is the status of function instance
type InstanceStatus struct {
	ID               int       `json:"id"`
	Running          bool      `json:"running"`
	LastReceivedTime time.Time `json:"lastReceivedTime"`
	LastQueryTime    time.Time `json:"lastQueryTime"`
	WorkerID         string    `json:"workerId"`
	MetadataWorkerID string    `json:"metadataWorkerId"`
}

// FunctionType is the object encapsulates all the function attributes
type FunctionType struct {
	Tenant       string                 `json:"tenant"`
	Namespace    string                 `json:"namespace"`
	FunctionName string                 `json:"functionName"`
	Component    string                 `json:"component"`
	Instances    map[int]InstanceStatus `json:"instances"`
	Parallism    int32                  `json:"parallism"`
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

// WriteFunctionMapIfNotExist writes a key/value to a thread safe map
func WriteFunctionMapIfNotExist(key string, f FunctionType) {
	fnMpLock.Lock()
	defer fnMpLock.Unlock()
	if _, ok := functionMap[key]; !ok {
		functionMap[key] = f
	}
}

// UpdateWorkerIDInFunctionMap updates the function worker ID against the key
func UpdateWorkerIDInFunctionMap(key, workerID string, instanceID int, running bool) {
	status := InstanceStatus{
		ID:               instanceID,
		Running:          running,
		LastQueryTime:    time.Now(),
		WorkerID:         workerID,
		MetadataWorkerID: workerID,
	}

	fnMpLock.Lock()
	defer fnMpLock.Unlock()
	if f, ok := functionMap[key]; ok {
		f.Instances[instanceID] = status
		functionMap[key] = f
	}
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
		ParseServiceRequest(sr.GetFunctionMetaData())
		// logger.Infof(" the total number of functions %d", len(functionMap))
	}
}

// ParseServiceRequest build a Function object based on Pulsar function metadata message
func ParseServiceRequest(sr *pb.FunctionMetaData) {
	fd := sr.FunctionDetails
	key := fd.GetTenant() + fd.GetNamespace() + fd.GetName()
	// allows to cache and display logs for service Type is pb.ServiceRequest_DELETE
	f := FunctionType{
		Tenant:       fd.GetTenant(),
		Namespace:    fd.GetNamespace(),
		FunctionName: fd.GetName(),
		Parallism:    fd.GetParallelism(),
		Component:    GetComponentType(fd.ComponentType),
		Instances:    make(map[int]InstanceStatus),
	}
	WriteFunctionMapIfNotExist(key, f)
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
func GetFunctionLog(functionName, workerID string, instanceID int, rd FunctionLogRequest) (FunctionLogResponse, error) {
	var fn FunctionType
	if workerID == "" {
		var err error
		fn, workerID, err = GetFunctionWorkerID(functionName, instanceID)
		if err != nil {
			return FunctionLogResponse{}, err
		}
	}

	// Set up a connection to the server.
	fqdn := workerID + functionWorkerDomain
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
		File:          logstream.FunctionLogPath(fn.Tenant, fn.Namespace, fn.FunctionName, strconv.Itoa(instanceID)),
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

// GetComponentType returns a component type in string (i.e. function, sink, source) based on protobuf component type
func GetComponentType(componentType pb.FunctionDetails_ComponentType) string {
	switch componentType {
	case pb.FunctionDetails_FUNCTION:
		return "functions"
	case pb.FunctionDetails_SOURCE:
		return "sources"
	case pb.FunctionDetails_SINK:
		return "sinks"
	default:
		return "unknown"
	}
}

// FuncInstanceStatus is the function/sink/source instance status
type FuncInstanceStatus struct {
	Running  bool   `json:"running"`
	Error    string `json:"error"`
	WorkerID string `json:"workerId"`
}

// FuncInstance is the function instance
type FuncInstance struct {
	InstanceID int                `json:"instanceId"`
	Status     FuncInstanceStatus `json:"status"`
}

// FuncStatus is the status for function instances
type FuncStatus struct {
	NumInstances int            `json:"numInstances"`
	NumRunning   int            `json:"numRunning"`
	Instances    []FuncInstance `json:"instances"`
}

// GetFunctionStatus get the function status
func GetFunctionStatus(fn FunctionType) (FuncStatus, error) {
	// util.Config.FunctionProxyURL
	functionRoute := fn.Tenant + "/" + fn.Namespace + "/" + fn.FunctionName + "/status"
	requestURL := util.SingleJoinSlash(util.Config.FunctionProxyURL,
		util.SingleJoinSlash("/admin/v3/"+fn.Component, functionRoute))
	log.Infof("GET FunctionStatus request url is %s", requestURL)

	// Update the headers to allow for SSL redirection
	newRequest, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return FuncStatus{}, err
	}
	newRequest.Header.Add("X-Request", "burnell-functions-cache")
	newRequest.Header.Add("Authorization", "Bearer "+util.Config.PulsarToken)

	client := &http.Client{
		CheckRedirect: util.PreserveHeaderForRedirect,
	}
	response, err := client.Do(newRequest)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		log.Errorf("%v", err)
		return FuncStatus{}, err
	}

	if response.StatusCode != http.StatusOK {
		return FuncStatus{}, fmt.Errorf("%s failure status code %d", requestURL, response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return FuncStatus{}, err
	}

	logger.Infof("nex to unmarshal res  body")
	status := FuncStatus{}
	if err := json.Unmarshal(body, &status); err != nil {
		return FuncStatus{}, err
	}

	logger.Infof("function status %v", status)
	return status, nil
}

const queryTimeout = 3 * time.Minute

// GetFunctionWorkerID gets function instance workerID
func GetFunctionWorkerID(functionName string, instanceID int) (FunctionType, string, error) {
	// var funcWorker string
	function, ok := ReadFunctionMap(functionName)
	if !ok {
		return FunctionType{}, "", ErrNotFoundFunction
	}
	instanceStatus, ok := function.Instances[instanceID]

	if ok && time.Since(instanceStatus.LastQueryTime) < queryTimeout {
		return function, instanceStatus.WorkerID, nil
	}

	logger.Infof("function type is %v", function)
	status, err := GetFunctionStatus(function)
	if err != nil {
		return FunctionType{}, "", err
	}

	for _, v := range status.Instances {
		if v.InstanceID == instanceID {
			UpdateWorkerIDInFunctionMap(functionName, v.Status.WorkerID, instanceID, v.Status.Running)
			return function, v.Status.WorkerID, nil
		}
	}

	return FunctionType{}, "", ErrNotFoundFunction
}

// /pulsar/logs/functions/ming-luo/namespace2/for-monitor-function
