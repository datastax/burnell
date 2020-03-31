package main

/**
 * The program to read and stream logs.
 */

import (
	"context"
	"log"
	"net"
	"os"

	pb "github.com/kafkaesque-io/burnell/src/logstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedLogStreamServer
}

const readStep int64 = 500

// FileReader is an object to keep track all the reader properties
type FileReader struct {
	forwardPos  int64
	backwardPos int64
	file        *os.File
}

// CreateFileReader creates a file reader object
func CreateFileReader(fileName string, forwardInitPos, backwardInitPos int64) (*FileReader, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	forwardPos := fileInfo.Size()
	backwardPos := fileInfo.Size()
	if forwardInitPos > 0 {
		forwardPos = forwardInitPos
	}
	if backwardInitPos > 0 {
		backwardPos = backwardInitPos
	}

	return &FileReader{
		forwardPos:  forwardPos,
		backwardPos: backwardPos,
		file:        file,
	}, nil
}

// Close closes the os File
func (f *FileReader) Close() {
	f.file.Close()
}

// ReadBackward reads backward
func (f *FileReader) ReadBackward(step int64) (string, int64, error) {
	bStep := readStep
	if step > 0 {
		bStep = readStep
	}
	buf := make([]byte, bStep)
	_, err := f.file.ReadAt(buf, f.backwardPos-readStep)
	if err != nil {
		return "", 0, err
	}
	f.backwardPos = f.backwardPos - readStep
	return string(buf), f.backwardPos, nil
}

// ReadForward reads forward
func (f *FileReader) ReadForward(step int64) (string, int64, error) {
	bStep := readStep
	if step > 0 {
		bStep = readStep
	}
	newEOFPos, err := f.file.Seek(0, 2)
	numNewBytes := newEOFPos - f.forwardPos
	if numNewBytes <= 0 {
		return "", 0, nil
	}

	buf := make([]byte, numNewBytes)
	_, err = f.file.ReadAt(buf, f.forwardPos)
	if err != nil {
		return "", 0, err
	}
	f.forwardPos = f.forwardPos + bStep
	return string(buf), f.backwardPos, nil
}

// Implementation of logStream server
func (s *server) Read(ctx context.Context, in *pb.ReadRequest) (*pb.LogLines, error) {
	r, err := CreateFileReader(in.GetFile(), in.GetForwardIndex(), in.GetBackwardIndex())
	if err != nil {
		return nil, err
	}

	var txt string
	if in.GetDirection() == pb.ReadRequest_BACKWARD {
		txt, _, err = r.ReadBackward(in.GetBytes())
		if err != nil {
			return nil, err
		}
		return &pb.LogLines{Logs: txt, ForwardIndex: r.forwardPos, BackwardIndex: r.backwardPos}, nil
	}
	txt, _, err = r.ReadForward(in.GetBytes())
	if err != nil {
		return nil, err
	}

	return &pb.LogLines{Logs: txt, ForwardIndex: r.forwardPos, BackwardIndex: r.backwardPos}, nil

}

func main() {
	listener, err := net.Listen("tcp", pb.LogServerPort)
	if err != nil {
		log.Fatalln(err)
	}

	srv := grpc.NewServer()
	pb.RegisterLogStreamServer(srv, &server{})
	reflection.Register(srv)

	if e := srv.Serve(listener); e != nil {
		log.Fatalln(err)
	}
}
