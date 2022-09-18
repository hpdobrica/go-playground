// Copyright 2019 The Oto Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build example
// +build example

package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/hajimehoshi/oto/v2"
)

var (
	sampleRate      = flag.Int("samplerate", 48000, "sample rate")
	channelCount    = flag.Int("channelcount", 2, "number of channel")
	bitDepthInBytes = flag.Int("bitdepthinbytes", 2, "bit depth in bytes")
)

type SineWave struct {
	freq   float64 // 523.3
	length int64   // 576000
	pos    int64   // 96000

	remaining []byte // []
}

func NewSineWave(freq float64, duration time.Duration) *SineWave {
	l := int64(*channelCount) * int64(*bitDepthInBytes) * int64(*sampleRate) * int64(duration) / int64(time.Second) // 576000
	// wtf?
	l = l / 4 * 4 // 576000
	return &SineWave{
		freq:   freq, // 523.3
		length: l,    // 576000
	}
}

// buf: [] length 96000
func (s *SineWave) Read(buf []byte) (int, error) {

	// if buffer is not %4, we need to make a bigger buffer which is to store all info (2 channels * 2 bytes for info)
	// if anything is remaining, fill the next buffer with it
	// (position was already moved when filling the bigger buffer)
	if len(s.remaining) > 0 { // |0>0|false|
		n := copy(buf, s.remaining)
		copy(s.remaining, s.remaining[n:])
		s.remaining = s.remaining[:len(s.remaining)-n]
		return n, nil
	}

	// if processed everything close
	if s.pos == s.length { // |0==57600|false|
		return 0, io.EOF
	}

	// if this will be the last you process, close at the end of this call
	//  reduce the buffer to the size of remaining info
	eof := false
	if s.pos+int64(len(buf)) > s.length { // |0+96000>576000|false|
		buf = buf[:s.length-s.pos]
		eof = true
	}

	// if buffer is not %4 create a slightly larger buffer that is and preserve the original one
	// so you can write info to it in the end (with a possible remainder that maybe wont fit in the original one)
	var origBuf []byte  // []
	if len(buf)%4 > 0 { // |96000%4>0|false|
		fmt.Println("buf not divisible by 4", len(buf))
		origBuf = buf
		buf = make([]byte, len(origBuf)+4-len(origBuf)%4)
	}

	length := float64(*sampleRate) / float64(s.freq) // |48000/523.3|91.72558|

	num := (*bitDepthInBytes) * (*channelCount) // |2*2|4|
	// p is tracking the position in the wave - if buffer is size 12, you will store 13th piece of wave into first place of buffer (i)
	p := s.pos / int64(num)   // |0/4|0| |1| |2|
	switch *bitDepthInBytes { // |2|
	case 1:
		for i := 0; i < len(buf)/num; i++ {
			const max = 127
			b := int(math.Sin(2*math.Pi*float64(p)/length) * 0.3 * max)
			for ch := 0; ch < *channelCount; ch++ {
				buf[num*i+ch] = byte(b + 128)
			}
			p++
		}
	case 2:
		for i := 0; i < len(buf)/num; i++ { // |i=0;i<24000;i++| |i=1| |i=2|
			const max = 32767                                             // max 16 bit signed int
			b := int16(math.Sin(2*math.Pi*float64(p)/length) * 0.3 * max) // |0| |0.068*0.3*32767=672.8335| |0.1365*0.3*32767=1342.51128|
			for ch := 0; ch < *channelCount; ch++ {
				// since b can be bigger than byte(255), casting to byte will give b%255
				// we keep b*2*2*2*2*2*2*2*2 in the next byte to tell us how much bigger the number is than 255
				// eg actual number ~= buf[0] + buf[1]*255 - something like that
				buf[num*i+2*ch] = byte(b)        // |buf[0]=0| |buf[2]=0| |buf[4]=byte(672.8335)| |buf[6]=byte(672.8335)| |buf[8]=byte(1342.51128)| |buf[10]=byte(1342.51128)|
				buf[num*i+1+2*ch] = byte(b >> 8) // |buf[1]=0| |buf[3]=0| |buf[5]=2.628255|       |buf[7]=2.628255|       |buf[9]=5.24418468|       |buf[11]=5.24418468|
			}
			p++
		}

	}

	// move position by the filled buffer
	s.pos += int64(len(buf)) // |96000|

	// if bigger buffer was created, fill it with what you can, and set the rest into remaining
	n := len(buf) // |96000|
	if origBuf != nil {
		n = copy(origBuf, buf) // |0|
		s.remaining = buf[n:]  // |[]|
	}

	if eof {
		return n, io.EOF
	}
	return n, nil
}

func play(context *oto.Context, freq float64, duration time.Duration) oto.Player {
	p := context.NewPlayer(NewSineWave(freq, duration))
	p.Play()
	return p
}

func run() error {
	const (
		freqC = 523.3
		freqE = 659.3
		freqG = 784.0
	)

	c, ready, err := oto.NewContext(*sampleRate, *channelCount, *bitDepthInBytes)
	if err != nil {
		return err
	}
	<-ready

	var wg sync.WaitGroup
	var players []oto.Player
	var m sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		p := play(c, freqC, 3*time.Second)
		m.Lock()
		players = append(players, p)
		m.Unlock()
		time.Sleep(3 * time.Second)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		p := play(c, freqE, 3*time.Second)
		m.Lock()
		players = append(players, p)
		m.Unlock()
		time.Sleep(3 * time.Second)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Second)
		p := play(c, freqG, 3*time.Second)
		m.Lock()
		players = append(players, p)
		m.Unlock()
		time.Sleep(3 * time.Second)
	}()

	wg.Wait()

	// Pin the players not to GC the players.
	runtime.KeepAlive(players)

	return nil
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		panic(err)
	}
}
