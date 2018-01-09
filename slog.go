package slog

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"
)

type now func() time.Time

type LogPrefixed interface {
	LogPrefix() string
}
type ReadAtWriter interface {
	io.ReaderAt
	io.Writer
}

var logging bool = true
var logMutex *sync.Mutex = &sync.Mutex{}
var dumpLogFile *os.File
var benchWriter io.Writer = os.Stdout
var benchclock = time.Now
var datestr string
var bytesRecorder *DumpRecorder

func SetBenchOutput(w io.Writer) {
	benchWriter = w
}

//func SetLogOutput(w io.Writer) {
func SetLogOutput(w io.Writer) {
	log.SetOutput(w)
}

func SetDumpRecorder(w io.Writer, sizeLoggingInterval time.Duration) *DumpRecorder {
	bytesRecorder = NewDumpRecorder(w, sizeLoggingInterval)
	return bytesRecorder
}

func SetChunkingDumpRecorder(sizeLoggingInterval time.Duration, chunkFilename string, chunker DumpChunker) (*DumpRecorder, error) {
	var err error
	bytesRecorder, err = NewChunkingDumpRecorder(sizeLoggingInterval, chunkFilename, chunker)
	if err != nil {
		return nil, err
	}
	return bytesRecorder, nil
}

//func nowDateString() string {
//	benchtime := time.Now()
//	y, mon, d := benchtime.Date()
//	h, min, s := benchtime.Clock()
//	return fmt.Sprintf("%02d%02d%02d-%02d%02d%02d", (y%100), mon, d, h, min, s)
//}

func SlogCreationTimeInString() string {
	if datestr == "" {
		benchtime := time.Now()
		y, mon, d := benchtime.Date()
		h, min, s := benchtime.Clock()
		datestr = fmt.Sprintf("%02d%02d%02d-%02d%02d%02d", (y % 100), mon, d, h, min, s)
	}
	return datestr
}

func SetLogOutputAsFile(filename string) (*os.File, error) {
	fullfilename := fmt.Sprintf("%s_%s.log", SlogCreationTimeInString(), filename)
	// Set logfile
	f, err := os.OpenFile(fullfilename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	SetLogOutput(f)
	return f, nil
}

func SetBenchOutputAsFile(filename string) (*os.File, error) {
	fullfilename := fmt.Sprintf("%s_%s.bch", SlogCreationTimeInString(), filename)
	// Set logfile
	f, err := os.OpenFile(fullfilename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	SetBenchOutput(f)
	return f, nil
}

func SetDumpRecorderAsFile(filename string, sizeLogginInterval time.Duration) (*DumpRecorder, error) {
	fullfilename := fmt.Sprintf("%s_%s.dump", SlogCreationTimeInString(), filename)
	// Set logfile
	f, err := os.OpenFile(fullfilename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	SetDumpRecorder(f, sizeLogginInterval)
	//return f, nil
	return bytesRecorder, nil
}

func SetBenchClock(clock now) {
	benchclock = clock
}

func EnableLogging() {
	logging = true
}
func DisableLogging() {
	logging = false
}

func logprefix(x interface{}) string {
	str, ok := x.(string)
	if ok {
		return str
	}
	switch x.(type) {
	case LogPrefixed:
		return x.(LogPrefixed).LogPrefix()
	default:
		switch reflect.TypeOf(x).Kind() {
		case reflect.Func:
			return "[" + runtime.FuncForPC(reflect.ValueOf(x).Pointer()).Name() + "]"
		case reflect.Struct:
			return "[" + reflect.ValueOf(x).Type().Name() + "]"
		default:
			return ""
		}
	}
}

func logToWriters(l string) {
	log.Print(l)
	if dumpLogFile != nil {
		logMutex.Lock()
		defer logMutex.Unlock()
		now := time.Now()
		y, mon, d := now.Date()
		h, min, s := now.Clock()
		dumpLogFile.Write([]byte(fmt.Sprintf("%4d/%02d/%02d %02d:%02d:%02d %s", y, mon, d, h, min, s, l)))
	}
}

func Logf(x interface{}, format string, v ...interface{}) {
	if !logging {
		return
	}
	logToWriters(fmt.Sprintf(logprefix(x)+" "+format, v...))
}

func Logln(x interface{}, v ...interface{}) {
	if !logging {
		return
	}
	if len(v) > 0 {
		logToWriters(fmt.Sprintln(append([]interface{}{logprefix(x)}, v...)...))
	} else {
		logToWriters(fmt.Sprintln(logprefix(x)))
	}
}

//func Fatalf(x interface{}, format string, v ...interface{}) {
//	fmt.Sprintf(logprefix(x)+" "+format, v...)
//}
//
//func Fatalln(x interface{}, v ...interface{}) {
//	if len(v) > 0 {
//		fmt.Sprintln(append([]interface{}{logprefix(x)}, v...)...)
//	} else {
//		fmt.Sprintln(logprefix(x))
//	}
//}

func benchprefix(x interface{}) string {
	now := benchclock()
	y, mon, d := now.Date()
	h, min, s := now.Clock()
	return fmt.Sprintf("%4d/%02d/%02d %02d:%02d:%02d %s", y, mon, d, h, min, s, logprefix(x))
}

func Benchf(x interface{}, format string, v ...interface{}) {
	s := fmt.Sprintf(benchprefix(x)+" "+format, v...)
	benchWriter.Write([]byte(s))
	fmt.Printf(s)
}

func Benchln(x interface{}, v ...interface{}) {
	if len(v) > 0 {
		s := fmt.Sprintln(append([]interface{}{benchprefix(x)}, v...)...)
		benchWriter.Write([]byte(s))
		fmt.Printf(s)
	} else {
		s := benchprefix(x)
		benchWriter.Write([]byte(s))
		fmt.Printf(s)
	}
}

func Stringify(x interface{}) string {
	marshaled, err := json.Marshal(x)
	if err == nil {
		return fmt.Sprintf("%T: %s", x, marshaled)
	}
	return fmt.Sprintf("%T: %v", x, x)
}

func StringifyIndent(x interface{}, indent string) string {
	marshaled, err := json.MarshalIndent(x, "", indent)
	if err == nil {
		return fmt.Sprintf("%T: %s", x, marshaled)
	}
	return fmt.Sprintf("%T: %v", x, x)
}

func Record(bytes []byte) {
	if bytesRecorder != nil {
		bytesRecorder.Record(bytes)
	}
}

type Dump struct {
	seq       int64     // 1
	timestamp time.Time // 2
	bytes     []byte    // 3
}

func DumpReader(src *os.File) (io.Reader, error) {
	//return src, nil
	ch, err := DumpChannel(src)
	if err != nil {
		return nil, err
	}
	var blob []byte
	for bytes := range ch {
		blob = append(blob, bytes...)
	}
	return bytes.NewReader(blob), nil
}
func DumpChannel(src *os.File) (chan []byte, error) {
	var buf []byte
	var err error
	var off int

	// read bytes
	fi, err := src.Stat()
	if err != nil {
		return nil, err
	}
	buf = make([]byte, fi.Size())
	src.Read(buf)
	bufsize := len(buf)

	deserializeRecord := func() *Dump {
		decodeInt := func() int32 {
			if err != nil {
				return 0
			}
			if off+4 > bufsize {
				err = errors.New("DecodeInt")
				return 0
			}
			x := binary.LittleEndian.Uint32(buf[off : off+4])
			off += 4
			return int32(x)
		}
		decodeLong := func() int64 {
			if err != nil {
				return 0
			}
			if off+8 > bufsize {
				err = errors.New("DecodeLong")
				return 0
			}
			x := int64(binary.LittleEndian.Uint64(buf[off : off+8]))
			off += 8
			return x
		}

		decodeDump := func(size int) []byte {
			if err != nil {
				return nil
			}
			if size < 1 {
				return nil
			}
			if off+size > bufsize {
				err = errors.New("DecodeDump")
				return nil
			}
			x := make([]byte, size)
			copy(x, buf[off:off+size])
			off += size
			return x
		}

		r := new(Dump)
		seq := decodeLong()
		timestamp := decodeLong()
		bytesize := decodeInt()
		r.seq = seq
		r.timestamp = time.Unix(0, timestamp)
		r.bytes = decodeDump(int(bytesize))
		if err != nil {
			return nil
		}
		return r
	}

	var records []*Dump
	for {
		r := deserializeRecord()
		if r == nil {
			break
		} else {
			records = append(records, r)
		}
	}
	//dst := make(chan *Record, len(records))
	dst := make(chan []byte, len(records))
	go func() {
		for _, r := range records {
			dst <- r.bytes
		}
		close(dst)
	}()
	return dst, nil
}

type DumpChunker struct {
	UnitInKB uint32
	Callback DumpChunkingCallback
}

const (
	DumpFileExtension = "dump"
	LogFileExtension  = "log"
)

// chunkFilename is the filename of the dump chunk and its log but extensions.
type DumpChunkingCallback func(dumpChunkFilename string, logChunkFilename string)

type DumpRecorder struct {
	dst             io.Writer
	seq             int64
	recording       bool
	chunker         DumpChunker
	chunking        bool
	chunkNum        uint
	chunkSize       uint64
	chunkFilename   string
	logChunkOffset  int64
	mutex           *sync.Mutex
	stopLoggingSize chan struct{}
}

func chunkedNumber(n uint64) string {
	str := fmt.Sprintf("%d", n)
	offset := len(str) % 3
	chunked := str[0:offset]
	for ; offset < len(str); offset = offset + 3 {
		chunked = chunked + "," + str[offset:offset+3]
	}
	return chunked
}
func NewChunkingDumpRecorder(sizeLoggingInterval time.Duration, chunkFilename string, chunker DumpChunker) (*DumpRecorder, error) {
	dumpChunkFilename := fmt.Sprintf("%s.%s.%d", chunkFilename, DumpFileExtension, 0)
	logChunkFilename := fmt.Sprintf("%s.%s.%d", chunkFilename, LogFileExtension, 0)
	dumpf, err := os.OpenFile(dumpChunkFilename, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("cannot open dump chunk file:", err)
	}
	logf, err := os.OpenFile(logChunkFilename, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("cannot open log chunk file:", err)
	}
	// critical section
	func() {
		logMutex.Lock()
		defer logMutex.Unlock()
		dumpLogFile = logf
	}()
	dr := NewDumpRecorder(dumpf, sizeLoggingInterval)
	dr.chunking = true
	dr.chunker = chunker
	dr.chunkFilename = chunkFilename
	return dr, nil
}
func NewDumpRecorder(w io.Writer, sizeLoggingInterval time.Duration) *DumpRecorder {
	br := new(DumpRecorder)
	br.dst = w
	br.recording = true
	br.stopLoggingSize = make(chan struct{})
	br.mutex = &sync.Mutex{}

	if sizeLoggingInterval != 0 {
		sizeLogPrefix := func() string {
			if f, ok := br.dst.(*os.File); ok {
				return fmt.Sprintf("Dump file, %s, ", f.Name())
			}
			return ""
		}
		go func() {
			for {
				select {
				case <-br.stopLoggingSize:
					Logf(br, "%ssize: %s B\n", sizeLogPrefix(), chunkedNumber(br.chunkSize))
					return
				case <-time.After(sizeLoggingInterval):
					Logf(br, "%ssize: %s B\n", sizeLogPrefix(), chunkedNumber(br.chunkSize))
				}
			}
		}()
	}
	return br
}
func (dr *DumpRecorder) Record(bytes []byte) {
	if dr.recording {
		dr.mutex.Lock()
		defer dr.mutex.Unlock()
		r := new(Dump)
		r.timestamp = time.Now()
		r.seq = dr.seq
		r.bytes = bytes

		// serialize
		var buf []byte
		encodeInt := func(i int32) {
			buf = append(buf, 0, 0, 0, 0)
			binary.LittleEndian.PutUint32(buf[len(buf)-4:], uint32(i))
		}
		encodeLong := func(l int64) {
			buf = append(buf, 0, 0, 0, 0, 0, 0, 0, 0)
			binary.LittleEndian.PutUint64(buf[len(buf)-8:], uint64(l))
		}
		encodeLong(r.seq)                         // seq
		encodeLong(int64(r.timestamp.UnixNano())) // timestamp
		encodeInt(int32(len(r.bytes)))            // bytes size
		buf = append(buf, r.bytes...)             // bytes
		if dr.chunking {
			appendedChunkSize := dr.chunkSize + uint64(len(buf))
			if appendedChunkSize > uint64(dr.chunker.UnitInKB)*1024 {
				// pass dump and log chunk files to callback
				dr.dst.(*os.File).Close()
				dumpLogFile.Close()
				dumpChunkFilename := fmt.Sprintf("%s.%s.%d", dr.chunkFilename, DumpFileExtension, dr.chunkNum)
				logChunkFilename := fmt.Sprintf("%s.%s.%d", dr.chunkFilename, LogFileExtension, dr.chunkNum)
				go dr.chunker.Callback(dumpChunkFilename, logChunkFilename)

				// start a new chunk
				dr.chunkNum = dr.chunkNum + 1
				newDumpChunkFilename := fmt.Sprintf("%s.%s.%d", dr.chunkFilename, DumpFileExtension, dr.chunkNum)
				newLogChunkFilename := fmt.Sprintf("%s.%s.%d", dr.chunkFilename, LogFileExtension, dr.chunkNum)
				newDumpChunkfile, err := os.OpenFile(newDumpChunkFilename, os.O_CREATE|os.O_WRONLY, 0666)
				if err != nil {
					Logf(dr, "Failed to open new dump chunk file, %s: %s\n", newDumpChunkFilename, err)
					dr.Close()
					return
				}
				newLogChunkfile, err := os.OpenFile(newLogChunkFilename, os.O_CREATE|os.O_WRONLY, 0666)
				if err != nil {
					Logf(dr, "Failed to open new dump chunk file, %s: %s\n", newDumpChunkFilename, err)
					dr.Close()
					return
				}

				// critical section
				func() {
					logMutex.Lock()
					defer logMutex.Unlock()
					dumpLogFile = newLogChunkfile
				}()
				dr.dst = newDumpChunkfile
				dr.chunkSize = 0
			}
		}
		dr.chunkSize += uint64(len(buf))

		// write to file
		dr.dst.Write(buf)
		dr.seq++
	}
}
func (dr *DumpRecorder) Enable() {
	dr.recording = true
}
func (dr *DumpRecorder) Disable() {
	dr.recording = false

}
func (dr *DumpRecorder) Close() {
	dr.Disable()
	close(dr.stopLoggingSize)
	switch dst := dr.dst.(type) {
	case *os.File:
		dst.Close()
	}
}
func (dr *DumpRecorder) DumpFile() (*os.File, bool) {
	switch f := dr.dst.(type) {
	case *os.File:
		return f, true
	default:
		return nil, false
	}
}
func (dr *DumpRecorder) LogPrefix() string {
	return "[DumpRecorder]"
}
func readAllFrom(rat io.ReaderAt, offset int64, bufsize int) ([]byte, error) {
	buf := make([]byte, bufsize)
	var n int
	var err error
	var accumed []byte
	var accumedn int64
	for err == nil {
		n, err = rat.ReadAt(buf, offset+accumedn)
		accumed = append(accumed, buf[0:n]...)
		accumedn += int64(n)
	}
	switch err {
	case io.EOF:
		return accumed[0:accumedn], nil
	default:
		return nil, err
	}
}
