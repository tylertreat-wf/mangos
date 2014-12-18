// Copyright 2014 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mangos

import (
	"math/rand"
	"sync"
	"time"
)

var pipes struct {
	byid   map[uint32]*pipe
	nextid uint32
	sync.Mutex
}

// pipe wraps the Pipe data structure with the stuff we need to keep
// for the core.  It implements the Endpoint interface.
type pipe struct {
	pipe   Pipe
	closeq chan struct{} // only closed, never passes data
	id     uint32
	index  int // index in master list of pipes for socket

	sock    *socket
	closing bool // true if we were closed
	sync.Mutex
}

func init() {
	pipes.byid = make(map[uint32]*pipe)
	pipes.nextid = uint32(rand.NewSource(time.Now().UnixNano()).Int63())
}

func newPipe(tranpipe Pipe, sock *socket) *pipe {
	p := &pipe{pipe: tranpipe}
	p.closeq = make(chan struct{})
	p.index = -1
	p.sock = sock
	for {
		pipes.Lock()
		p.id = pipes.nextid & 0x7fffffff
		pipes.nextid++
		if p.id != 0 && pipes.byid[p.id] == nil {
			pipes.byid[p.id] = p
			pipes.Unlock()
			break
		}
		pipes.Unlock()
	}
	return p
}

func (p *pipe) GetID() uint32 {
	pipes.Lock()
	defer pipes.Unlock()
	return p.id
}

func (p *pipe) Close() error {
	p.Lock()
	if p.closing {
		return nil
	}
	p.closing = true
	p.Unlock()
	close(p.closeq)
	p.sock.remPipe(p)
	p.pipe.Close()
	pipes.Lock()
	delete(pipes.byid, p.id)
	p.id = 0 // safety
	pipes.Unlock()
	return nil
}

func (p *pipe) SendMsg(msg *Message) error {

	if err := p.pipe.Send(msg); err != nil {
		p.Close()
		return err
	}
	return nil
}

func (p *pipe) RecvMsg() *Message {

	msg, err := p.pipe.Recv()
	if err != nil {
		p.Close()
		return nil
	}
	msg.Remote = p.pipe.RemoteAddr()
	return msg
}
