// Copyright 2017 The Minigmp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package minigmp

import (
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/cznic/crt"
)

func caller(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(2)
	fmt.Fprintf(os.Stderr, "# caller: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	_, fn, fl, _ = runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# \tcallee: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func dbg(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# dbg %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func TODO(...interface{}) string { //TODOOK
	_, fn, fl, _ := runtime.Caller(1)
	return fmt.Sprintf("# TODO: %s:%d:\n", path.Base(fn), fl) //TODOOK
}

func use(...interface{}) {}

func init() {
	use(caller, dbg, TODO) //TODOOK
}

// ============================================================================

func TestAdd(t *testing.T) {
	const (
		nTests  = 100
		nDigits = 1000
	)

	tls := crt.NewTLS()

	defer tls.Close()

	var ba, bb [nDigits]byte
	for i := 0; i < nTests; i++ {
		for i := range ba {
			ba[i] = byte('0' + rand.Intn(10))
			bb[i] = byte('0' + rand.Intn(10))
		}

		func() {
			var r, x, y [1]Xmpz_srcptr
			Xmpz_init(tls, &r)
			Xmpz_init(tls, &x)
			Xmpz_init(tls, &y)
			sa := string(ba[:])
			sb := string(bb[:])
			ca := crt.CString(sa)
			cb := crt.CString(sb)

			defer func() {
				Xmpz_clear(tls, &r)
				Xmpz_clear(tls, &x)
				Xmpz_clear(tls, &y)
				crt.Free(ca)
				crt.Free(cb)
			}()

			Xmpz_set_str(tls, &x, (*int8)(ca), 10)
			Xmpz_set_str(tls, &y, (*int8)(cb), 10)
			Xmpz_add(tls, &r, &x, &y)
			cr := Xmpz_get_str(tls, nil, 10, &r)

			defer crt.Free(unsafe.Pointer(cr))

			bigX, _ := big.NewInt(0).SetString(sa, 10)
			bigY, _ := big.NewInt(0).SetString(sb, 10)
			bigX.Add(bigX, bigY)
			if g, e := crt.GoString(cr), bigX.String(); g != e {
				t.Fatal("%v + %s = %v, got %v", sa, sb, e, g)
			}
		}()
	}
}
