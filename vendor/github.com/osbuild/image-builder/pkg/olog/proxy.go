package olog

import "io"

// Print is a proxy for log.Print.
func Print(v ...any) {
	Default().Print(v...)
}

// Printf is a proxy for log.Printf.
func Printf(format string, v ...any) {
	Default().Printf(format, v...)
}

// Println is a proxy for log.Println.
func Println(v ...any) {
	Default().Println(v...)
}

// Fatal is a proxy for log.Fatal.
func Fatal(v ...any) {
	Default().Fatal(v...)
}

// Fatalf is a proxy for log.Fatalf.
func Fatalf(format string, v ...any) {
	Default().Fatalf(format, v...)
}

// Fatalln is a proxy for log.Fatalln.
func Fatalln(v ...any) {
	Default().Fatalln(v...)
}

// Panic is a proxy for log.Panic.
func Panic(v ...any) {
	Default().Panic(v...)
}

// Panicf is a proxy for log.Panicf.
func Panicf(format string, v ...any) {
	Default().Panicf(format, v...)
}

// Panicln is a proxy for log.Panicln.
func Panicln(v ...any) {
	Default().Panicln(v...)
}

// Writer is a proxy for log.Writer.
func Writer() io.Writer {
	return Default().Writer()
}

// Output is a proxy for log.Output.
func Output(calldepth int, s string) error {
	return Default().Output(calldepth+1, s)
}

// SetOutput is a proxy for log.SetOutput.
func SetOutput(w io.Writer) {
	Default().SetOutput(w)
}

// Flags is a proxy for log.Flags.
func Flags() int {
	return Default().Flags()
}

// SetFlags is a proxy for log.SetFlags.
func SetFlags(flag int) {
	Default().SetFlags(flag)
}

// Prefix is a proxy for log.Prefix.
func Prefix() string {
	return Default().Prefix()
}

// SetPrefix is a proxy for log.SetPrefix.
func SetPrefix(prefix string) {
	Default().SetPrefix(prefix)
}
