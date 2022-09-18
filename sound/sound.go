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

type Sound struct {
	freq     float64 // 523.3
	length   int64   // 576000
	pos      int64   // 96000
	function func(int64, float64) float64

	remaining []byte // []
}

var (
	sampleRate      = flag.Int("samplerate", 48000, "sample rate")
	channelCount    = flag.Int("channelcount", 2, "number of channel")
	bitDepthInBytes = flag.Int("bitdepthinbytes", 2, "bit depth in bytes")
)

func NewSound(freq float64, duration time.Duration, function func(int64, float64) float64) *Sound {
	l := int64(*channelCount) * int64(*bitDepthInBytes) * int64(*sampleRate) * int64(duration) / int64(time.Second)

	return &Sound{
		freq:     freq,
		length:   l,
		function: function,
	}
}

func (s *Sound) Read(buf []byte) (int, error) {

	// if buffer is not %4, we need to make a bigger buffer which is to store all info (2 channels * 2 bytes for info)
	// if anything is remaining, fill the next buffer with it
	// (position was already moved when filling the bigger buffer)
	if len(s.remaining) > 0 {
		n := copy(buf, s.remaining)
		copy(s.remaining, s.remaining[n:])
		s.remaining = s.remaining[:len(s.remaining)-n]
		return n, nil
	}

	// if processed everything close
	if s.pos == s.length {
		return 0, io.EOF
	}

	// if this will be the last you process, close at the end of this call
	//  reduce the buffer to the size of remaining info
	eof := false
	if s.pos+int64(len(buf)) > s.length {
		buf = buf[:s.length-s.pos]
		eof = true
	}

	// if buffer is not %4 create a slightly larger buffer that is and preserve the original one
	// so you can write info to it in the end (with a possible remainder that maybe wont fit in the original one)
	var origBuf []byte
	if len(buf)%4 > 0 {
		fmt.Println("buf not divisible by 4", len(buf))
		origBuf = buf
		buf = make([]byte, len(origBuf)+4-len(origBuf)%4)
	}

	sampleFrequency := float64(*sampleRate) / float64(s.freq)

	num := (*bitDepthInBytes) * (*channelCount)
	// p is tracking the position in the wave - if buffer is size 12, you will store 13th piece of wave into first place of buffer (i)
	p := s.pos / int64(num)
	switch *bitDepthInBytes {
	case 1:
		for i := 0; i < len(buf)/num; i++ {
			const max = 127
			b := int(s.function(p, sampleFrequency) * 0.3 * max)
			for ch := 0; ch < *channelCount; ch++ {
				buf[num*i+ch] = byte(b + 128)
			}
			p++
		}
	case 2:
		for i := 0; i < len(buf)/num; i++ {
			const max = 32767 // max 16 bit signed int
			// b := int16(math.Sin(2*math.Pi*float64(p)/sampleFrequency) * 0.3 * max)
			b := int16(s.function(p, sampleFrequency) * 0.3 * max)
			for ch := 0; ch < *channelCount; ch++ {
				// since b can be bigger than byte(255), casting to byte will give b%255
				// we keep b*2*2*2*2*2*2*2*2 in the next byte to tell us how much bigger the number is than 255
				// eg actual number ~= buf[0] + buf[1]*255 - something like that
				buf[num*i+2*ch] = byte(b)
				buf[num*i+1+2*ch] = byte(b >> 8)
			}
			p++
		}

	}

	// move position by the filled buffer
	s.pos += int64(len(buf))

	// if bigger buffer was created, fill it with what you can, and set the rest into remaining
	n := len(buf)
	if origBuf != nil {
		n = copy(origBuf, buf)
		s.remaining = buf[n:]
	}

	if eof {
		return n, io.EOF
	}
	return n, nil
}

func play(context *oto.Context, freq float64, duration time.Duration) oto.Player {
	p := context.NewPlayer(NewSound(freq, duration, func(i int64, f float64) float64 {
		return math.Sin(2 * math.Pi * float64(i) / f)
	}))
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
