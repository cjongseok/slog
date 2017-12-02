package slog

import (
	"fmt"
	"log"
	"encoding/json"
	"reflect"
	"runtime"
	"os"
	"time"
)

type now func() time.Time

type LogPrefixed interface {
	LogPrefix() string
}

var logging bool = true
var benchfile *os.File = os.Stdout
var benchclock = time.Now

func SetBenchOutput(f *os.File) {
	benchfile = f
}

func SetLogOutput(f *os.File) {
	log.SetOutput(f)
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

