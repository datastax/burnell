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

package route

import (
	"errors"
)

// Sema the semaphore object used internally
type Sema struct {
	Size int
	Ch   chan int
}

// NewSema creates a new semaphore
func NewSema(Length int) Sema {
	obj := Sema{}
	InitChan := make(chan int, Length)
	obj.Size = Length
	obj.Ch = InitChan
	return obj
}

// Acquire aquires a semaphore lock
func (s *Sema) Acquire() error {
	select {
	case s.Ch <- 1:
		return nil
	default:
		return errors.New("all semaphore buffer full")
	}
}

// Release release a semaphore lock
func (s *Sema) Release() error {
	select {
	case <-s.Ch:
		return nil
	default:
		return errors.New("all semaphore buffer empty")
	}
}
