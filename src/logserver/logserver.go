 //
 //  Copyright (c) 2021 Datastax, Inc.
 //  
 //  Licensed to the Apache Software Foundation (ASF) under one
 //  or more contributor license agreements.  See the NOTICE file
 //  distributed with this work for additional information
 //  regarding copyright ownership.  The ASF licenses this file
 //  to you under the Apache License, Version 2.0 (the
 //  "License"); you may not use this file except in compliance
 //  with the License.  You may obtain a copy of the License at
 //  
 //     http://www.apache.org/licenses/LICENSE-2.0
 //  
 //  Unless required by applicable law or agreed to in writing,
 //  software distributed under the License is distributed on an
 //  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 //  KIND, either express or implied.  See the License for the
 //  specific language governing permissions and limitations
 //  under the License.
 //

package main

/**
 * The program to read and stream logs.
 */

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	pb "github.com/datastax/burnell/src/logstream"
	"github.com/datastax/burnell/src/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedLogStreamServer
}

const readStep int64 = 2400

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
	readBytes := readStep
	if step > readStep {
		readBytes = step
	}

	readPos := f.backwardPos - readBytes
	// fmt.Printf("readPos %d origin backwardPos %d readBytes %d\n", readPos, f.backwardPos, readBytes)
	if readPos <= 0 {
		temp := f.forwardPos
		f.forwardPos = 0
		text, _, err := f.ReadForward(f.backwardPos)
		f.forwardPos = temp
		if err != nil {
			return "", f.backwardPos, err
		}
		//convert the forwardPos to backwarPos since we have reached the beginning of the file
		return text, 0, nil
	}

	process := true
	for process {
		buf := make([]byte, readBytes)
		_, err := f.file.ReadAt(buf, f.backwardPos-readBytes)
		if err != nil {
			return "", 0, err
		}
		if logs, index := truncatePreLine(string(buf)); index > 0 {
			// fmt.Printf("index %d readPos %d origin backwardPos %d readBytes %d\n", index, readPos, f.backwardPos, readBytes)
			return logs, f.backwardPos - readBytes + int64(index), nil
		}
		readBytes = readBytes + readStep
	}
	return "", f.backwardPos, fmt.Errorf("unexpected read backwards error")
}

// ReadForward reads forward
func (f *FileReader) ReadForward(step int64) (string, int64, error) {
	readBytes := readStep
	if step > readStep {
		readBytes = step
	}

	newEOFPos, err := f.file.Seek(0, 2)
	numNewBytes := newEOFPos - f.forwardPos
	if numNewBytes <= 0 {
		return "", 0, nil
	}

	process := true
	for process {
		if numNewBytes < readBytes {
			readBytes = numNewBytes
		}
		buf := make([]byte, readBytes)
		_, err = f.file.ReadAt(buf, f.forwardPos)
		if err != nil {
			return "", 0, err
		}

		if logs, index := truncatePostLine(string(buf)); index > 0 {
			return logs, f.forwardPos + int64(index), nil
		}

		readBytes = readBytes + readStep
		log.Printf("more readyBytes %d\n", readBytes)
	}

	return "", newEOFPos, fmt.Errorf("unexpected error")
}

// Implementation of logStream server
func (s *server) Read(ctx context.Context, in *pb.ReadRequest) (*pb.LogLines, error) {
	r, err := CreateFileReader(in.GetFile(), in.GetForwardIndex(), in.GetBackwardIndex())
	if err != nil {
		return nil, err
	}

	var txt string
	if in.GetDirection() == pb.ReadRequest_BACKWARD {
		txt, newBackwardPos, err := r.ReadBackward(in.GetBytes())
		if err != nil {
			return nil, err
		}
		// fmt.Printf("backwardPos %d forwardPos %d\ntxt:%s\n", newBackwardPos, r.forwardPos, txt)
		return &pb.LogLines{Logs: txt, ForwardIndex: r.forwardPos, BackwardIndex: newBackwardPos}, nil
	}
	txt, newForwardPos, err := r.ReadForward(in.GetBytes())
	if err != nil {
		return nil, err
	}

	return &pb.LogLines{Logs: txt, ForwardIndex: newForwardPos, BackwardIndex: r.backwardPos}, nil
}

func main() {
	port := util.AssignString(util.GetConfig().LogServerPort, os.Getenv("LogServerPort"), pb.DefaultLogServerPort)
	fmt.Printf("starting log server on port %s, log path prefix %s\n", port, pb.FilePath)
	listener, err := net.Listen("tcp", port)
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

// truncatePostLine ensures complete lines for forward walking/log tailing
func truncatePostLine(text string) (string, int) {
	if !strings.Contains(text, "\n") {
		return "", -1
	}
	offset := strings.LastIndex(text, "\n")
	lines := text[:offset+1]
	return lines, offset + 1
}

// truncatePreLine ensures complete lines for backward walking
func truncatePreLine(text string) (string, int) {
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		// we need at least two complete lines
		return "", -1
	}
	offset := strings.Index(text, "\n")
	return text[offset+1:], offset + 1
}
