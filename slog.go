package slog

import (
	"fmt"
	"log"
	"encoding/json"
	"reflect"
	"runtime"
	"os"
	"time"
	"encoding/binary"
	"errors"
	"io"
	"bytes"
)

type now func() time.Time

type LogPrefixed interface {
	LogPrefix() string
}

var logging bool = true
var benchfile *os.File = os.Stdout
var benchclock = time.Now
var datestr string
var bytesRecorder *DumpRecorder

func SetBenchOutput(f *os.File) {
	benchfile = f
}

func SetLogOutput(f *os.File) {
	log.SetOutput(f)
}

func SetDumpRecorder(f *os.File) {
	bytesRecorder = NewDumpRecorder(f)
}

func nowDateString() string {
	benchtime := time.Now()
	y, mon, d := benchtime.Date()
	h, min, s := benchtime.Clock()
	return fmt.Sprintf("%02d%02d%02d-%02d%02d%02d", (y%100), mon, d, h, min, s)
}

func SetLogOutputAsFile(filename string) (*os.File, error) {
	if datestr == "" {
		datestr = nowDateString()
	}
	fullfilename := fmt.Sprintf("%s_%s.log", datestr, filename)
	// Set logfile
	f, err := os.OpenFile(fullfilename, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	SetLogOutput(f)
	return f, nil
}

func SetBenchOutputAsFile(filename string) (*os.File, error) {
	if datestr == "" {
		datestr = nowDateString()
	}
	fullfilename := fmt.Sprintf("%s_%s.bch", datestr, filename)
	// Set logfile
	f, err := os.OpenFile(fullfilename, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	SetBenchOutput(f)
	return f, nil
}

func SetDumpRecorderAsFile(filename string) (*DumpRecorder, error) {
	if datestr == "" {
		datestr = nowDateString()
	}
	fullfilename := fmt.Sprintf("%s_%s.dump", datestr, filename)
	// Set logfile
	f, err := os.OpenFile(fullfilename, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	SetDumpRecorder(f)
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
		switch reflect.TypeOf(x).Kind(){
		case reflect.Func:
			return "[" + runtime.FuncForPC(reflect.ValueOf(x).Pointer()).Name() + "]"
		case reflect.Struct:
			return "[" + reflect.ValueOf(x).Type().Name() + "]"
		default:
			return ""
		}
	}
}

func Logf(x interface{}, format string, v ...interface{}) {
	if !logging {
		return
	}
	log.Printf(logprefix(x) + " " + format, v...)
}

func Logln(x interface{}, v ...interface{}) {
	if !logging {
		return
	}
	if len(v) > 0 {
		log.Println(append([]interface{}{logprefix(x)} , v...)...)
	} else {
		log.Println(logprefix(x))
	}
	//logf(x, format + "\n", v...)
}

func Fatalf(x interface{}, format string, v ...interface{}) {
	log.Fatalf(logprefix(x) + " " + format, v...)
}

func Fatalln(x interface{}, v ...interface{}) {
	if len(v) > 0 {
		log.Fatalln(append([]interface{}{logprefix(x)}, v...)...)
	} else {
		log.Fatalln(logprefix(x))
	}
}

func benchprefix(x interface{}) string {
	now := benchclock()
	y, mon ,d := now.Date()
	h, min, s := now.Clock()
	return fmt.Sprintf("%4d/%02d/%02d %02d:%02d:%02d %s",y, mon, d, h, min, s, logprefix(x))
}

func Benchf(x interface{}, format string, v ...interface{}) {
	s := fmt.Sprintf(benchprefix(x) + " " + format, v...)
	benchfile.WriteString(s)
	fmt.Printf(s)
}

func Benchln(x interface{}, v ...interface{}) {
	if len(v) > 0 {
		s := fmt.Sprintln(append([]interface{}{benchprefix(x)} , v...)...)
		benchfile.WriteString(s)
		fmt.Printf(s)
	} else {
		s := benchprefix(x)
		benchfile.WriteString(s)
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
	marshaled, err := json.MarshalIndent(x, "" ,indent)
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
	seq			int64		// 1
	timestamp 	time.Time 	// 2
	bytes 		[]byte		// 3
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
		decodeInt := func () int32{
			if err != nil{
				return 0
			}
			if off+4 > bufsize{
				err = errors.New("DecodeInt")
				return 0
			}
			x := binary.LittleEndian.Uint32(buf[off: off+4])
			off += 4
			return int32(x)
		}
		decodeLong := func () int64{
			if err != nil{
				return 0
			}
			if off+8 > bufsize{
				err = errors.New("DecodeLong")
				return 0
			}
			x := int64(binary.LittleEndian.Uint64(buf[off: off+8]))
			off += 8
			return x
		}

		decodeDump := func (size int) []byte{
			if err != nil{
				return nil
			}
			if size < 1 {
				return nil
			}
			if off+size > bufsize{
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

type DumpRecorder struct {
	src 		chan []byte
	dst 		*os.File
	seq 		int64
	recording 	bool
}
func NewDumpRecorder(dst *os.File) *DumpRecorder {
	br := new(DumpRecorder)
	br.src = make(chan []byte)
	br.dst = dst
	br.seq = 0
	br.recording = true
	return br
}
func (br *DumpRecorder) Record(bytes []byte) {
	if (br.recording) {
		r := new(Dump)
		r.timestamp = time.Now()
		r.seq = br.seq
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
		encodeLong(r.seq)							// seq
		encodeLong(int64(r.timestamp.UnixNano()))	// timestamp
		encodeInt(int32(len(r.bytes)))				// bytes size
		buf = append(buf, r.bytes...)				// bytes

		// write to file
		br.dst.Write(buf)
		br.seq++
	}
}
func (br *DumpRecorder) Enable() {
	br.recording = true
}
func (br *DumpRecorder) Disable() {
	br.recording = false

}
func (br *DumpRecorder) Close() {
	br.Disable()
	close(br.src)
	br.dst.Close()
}
