package main

import(
	"io"
	"fmt"
	"os"
	"flag"
	"strconv"
	"strings"
	"math/rand"
)

var helpOption bool
var versionOption bool
var verboseOption bool
func init() {
	flag.BoolVar(&helpOption, "help", false, "show usage");
	flag.BoolVar(&versionOption, "version", false, "show version");
	flag.BoolVar(&verboseOption, "verbose", false, "be verbose about changes");
}
type spec struct {
	offset uint64
	length uint64
	rand   bool // bit ignored
	bitset bool
	bitdel bool
	bitinv bool
	bit    *uint8
	value  *uint8
}

var specs []spec
var specidx int
var curspec *spec
var togo uint64
var totalPos uint64

func errorMaker(c byte) byte {
	if curspec.value!=nil {
		return *curspec.value
	}
	if curspec.rand {
		return byte(rand.Intn(256))
	}
	// some bit op
	var bit int
	if curspec.bit!=nil {
		bit=int(*curspec.bit)
	} else {
		bit=rand.Intn(8)
	}
	if curspec.bitinv {
		var d byte
		d=1<<bit
		return c ^ d
	}
	if curspec.bitset {
		var d byte
		d=1<<bit
		return c | d
	}
	// del
	var d byte
	d=1<<bit
	return c &^ d;
}

func errorMakerLoop(buf []byte, n uint64) []byte {
	var i uint64
	for {
		if n-i<togo {
			togo-=n-i
			return buf
		}
		i+=togo
		old:=buf[i]
		buf[i]=errorMaker(buf[i])
		if verboseOption {
			fmt.Fprintf(os.Stderr, "changed %v to %v at %v\n",old,buf[i],totalPos+i)
		}

		specidx++
		if specidx>=len(specs) {
			specidx=0
		}
		curspec=&specs[specidx]
		togo=curspec.offset
		if curspec.length>0 {
			togo+=uint64(rand.Int63n(int64(curspec.length)))
		}
	}
}

func parseSpecs(args []string) {
	specs=make([]spec,0)
	for _, a := range args {
		var S spec
		i:=strings.Index(a,":")
		if -1==i {
			fmt.Fprintf(os.Stderr, "bad specification %s: no colon.\n",a)
			os.Exit(2)
		}
		if 0==i {
			fmt.Fprintf(os.Stderr, "bad specification %s: colon at start.\n",a)
			os.Exit(2)
		}
		p1:=a[:i]
		if p1=="r" || p1=="R" {
			S.rand=true
		} else if (p1[0]=='b') {
			if len(p1)<3 {
				fmt.Fprintf(os.Stderr, "bad specification %s: incomplete.\n",a)
			}
			if p1[1]=='+' {
				S.bitset=true
			} else if p1[1]=='-' {
				S.bitdel=true
			} else if p1[1]=='^' {
				S.bitinv=true
			} else {
				fmt.Fprintf(os.Stderr, "bad specification %s: bad bit operation.\n",a)
				os.Exit(2)
			}
			if p1[2]!='r' {
				t, err:=strconv.ParseUint(p1[2:],10,64)
				if err != nil || t>7{
					fmt.Fprintf(os.Stderr, "bad specification %s: bad bit number.\n",a)
					os.Exit(2)
				}
				S.bit=new(uint8)
				*S.bit=uint8(t)
			}
		} else {
			t, err:=strconv.ParseUint(p1,10,64)
			if err != nil || t>255{
				fmt.Fprintf(os.Stderr, "bad specification %s: bad value.\n",a)
				os.Exit(2)
			}
			S.value=new(uint8)
			*S.value=uint8(t)
		}
		b:=a[i+1:]

		i=strings.Index(b,":")
		if 0==i {
			fmt.Fprintf(os.Stderr, "bad specification %s: double colon.\n",a)
			os.Exit(2)
		} else if i>0 {
			p2:=b[:i]
			p3:=b[i+1:]
			t, err:=strconv.ParseUint(p2,10,64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "bad specification %s: bad offset.\n",a)
				os.Exit(2)
			}
			S.offset=t
			t, err=strconv.ParseUint(p3,10,64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "bad specification %s: bad length.\n",a)
				os.Exit(2)
			}
			S.length=t
		} else {
			t, err:=strconv.ParseUint(b,10,64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "bad specification %s: bad offset.\n",a)
				os.Exit(2)
			}
			S.offset=t
			S.length=0
		}
		specs=append(specs,S)
	}
	curspec=&specs[0]
	togo=curspec.offset
	if curspec.length>0 {
		togo+=uint64(rand.Int63n(int64(curspec.length)))
	}

}
func usage() {
	fmt.Print(`
Usage: lrzsz-gen-errors [options] errspec...
Generate errors while copying from stdin to stdout.

errspec... is a series of errspecs.
An errspec consists of two or more elements, separated by colons: C:n[:m].

C is either:
  a number: set the byte to that number (0..255).
  r or R: switch some random bit.
  b0 to b7: switch that bit (b with a digit).

n is a number, denoting the distance to the error, and 
m is another number, describing a random range of bytes where the error will 
happen. n in that case describes the start of the range, m the length.

If the end of the errspecs is reached before input ended, the program will 
restart at the first errspec.

Examples:
lrzsz-gen-errors 0:1024 255:1024
    will set byte 1024 to 0, 2048 to 255, 3072 to 0, ...
lrzsz-gen-errors b7:512
    will switch bit 7 on every 512th byte.
lrzsz-gen-errors r:512:8192
    will switch a random bit in a random byte of the 8192 byte following 
    the next 512 bytes.
	`)
}
func main() {
	flag.Parse()
	if versionOption {
		fmt.Printf("lrzsz-gen-errors (GNU lrzsz) 0.0.0\n")
		fmt.Printf("(C) 2020 bla\n")
		os.Exit(0)
	}
	if helpOption {
		usage()
		os.Exit(0)
	}
	bufsize:=65536
	buf:=make([]byte,bufsize,bufsize);
	parseSpecs(flag.Args())
	for {
		buf=buf[:bufsize]
		n, err := os.Stdin.Read(buf)
		// fmt.Fprintf(os.Stderr, "read returned %d %v\n", n, err);
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr,"read: %s\n");
				os.Exit(1);
			}
		}
		if n!=0 {
			errorMakerLoop(buf,uint64(n))
			buf=buf[:n]
			_, err2:=os.Stdout.Write(buf)
			// fmt.Fprintf(os.Stderr, "write returned %d %v\n", m, err2);
			if err2 != nil {
				fmt.Fprintf(os.Stderr,"write: %s\n");
				os.Exit(1);
			}

		}
		totalPos+=uint64(n)
		if err==io.EOF {
			break
		}
	}
}