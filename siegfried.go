// Copyright 2014 Richard Lehane. All rights reserved.
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

// Package siegfried identifies file formats
//
// Example:
//  s, err := siegfried.Load("pronom.sig")
//  if err != nil {
//  	log.Fatal(err)
//  }
//  f, err := os.Open("file")
//  if err != nil {
//  	log.Fatal(err)
//  }
//  defer f.Close()
//  c, err := s.Identify("filename", f)
//  if err != nil {
//  	log.Fatal(err)
//  }
//  for id := range c {
//  	fmt.Print(id)
//  }
package siegfried

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/richardlehane/siegfried/config"
	"github.com/richardlehane/siegfried/pkg/core"
	"github.com/richardlehane/siegfried/pkg/core/bytematcher"
	"github.com/richardlehane/siegfried/pkg/core/containermatcher"
	"github.com/richardlehane/siegfried/pkg/core/extensionmatcher"
	"github.com/richardlehane/siegfried/pkg/core/persist"
	"github.com/richardlehane/siegfried/pkg/core/siegreader"
	"github.com/richardlehane/siegfried/pkg/pronom"
)

// Siegfried structs are persisent objects that can be serialised to disk.
// The private fields are the three matchers (extension, container, byte) and the identifiers.
type Siegfried struct {
	C  time.Time    // signature create time
	em core.Matcher // extensionmatcher
	cm core.Matcher // containermatcher
	bm core.Matcher // bytematcher
	// mutatable fields follow
	ids     []core.Identifier // identifiers
	buffers *siegreader.Buffers
}

// New creates a new Siegfried struct. It sets the create time to time.Now() and initializes the three matchers.
//
// Example:
//  s := New()
//  p, err := pronom.New() // create a new PRONOM identifier
//  if err != nil {
//  	// handle err
//  }
//  err = s.Add(p) // add the identifier to the Siegfried
//  if err != nil {
//  	// handle err
//  }
//  err = s.Save("pronom.sig") // save the Siegfried
//  if err != nil {
//  	// handle err
//  }
func New() *Siegfried {
	s := &Siegfried{}
	s.C = time.Now()
	s.em = extensionmatcher.New()
	s.cm = containermatcher.New()
	s.bm = bytematcher.New()
	s.buffers = siegreader.New()
	return s
}

// Add adds an identifier to a Siegfried struct.
// The identifer is type switched to test if it is supported. At present, only PRONOM identifiers are supported
func (s *Siegfried) Add(i core.Identifier) error {
	switch i := i.(type) {
	default:
		return fmt.Errorf("siegfried: unknown identifier type %T", i)
	case *pronom.Identifier:
		if err := i.Add(s.em); err != nil {
			return err
		}
		if err := i.Add(s.cm); err != nil {
			return err
		}
		if err := i.Add(s.bm); err != nil {
			return err
		}
		s.ids = append(s.ids, i)
	}
	return nil
}

// Save persists a Siegfried struct to disk (path)
func (s *Siegfried) Save(path string) error {
	ls := persist.NewLoadSaver(nil)
	ls.SaveString("siegfried")
	ls.SaveTime(s.C)
	s.em.Save(ls)
	s.cm.Save(ls)
	s.bm.Save(ls)
	ls.SaveTinyUInt(len(s.ids))
	for _, i := range s.ids {
		i.Save(ls)
	}
	if ls.Err != nil {
		return ls.Err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(config.Magic())
	if err != nil {
		return err
	}
	z, err := flate.NewWriter(f, 1)
	if err != nil {
		return err
	}
	_, err = z.Write(ls.Bytes())
	if err != nil {
		return err
	}
	z.Close()
	return nil
}

// Load creates a Siegfried struct and loads content from path
func Load(path string) (*Siegfried, error) {
	errOpening := "siegfried: error opening signature file; got %v\nTry running `sf -update`"
	errNotSig := "siegfried: not a siegfried signature file"
	fbuf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(errOpening, err)
	}
	if string(fbuf[:len(config.Magic())]) != string(config.Magic()) {
		return nil, fmt.Errorf(errNotSig)
	}
	r := bytes.NewBuffer(fbuf[len(config.Magic()):])
	rc := flate.NewReader(r)
	defer rc.Close()
	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf(errOpening, err)
	}
	ls := persist.NewLoadSaver(buf)
	if ls.LoadString() != "siegfried" {
		return nil, fmt.Errorf(errNotSig)
	}
	return &Siegfried{
		C:  ls.LoadTime(),
		em: extensionmatcher.Load(ls),
		cm: containermatcher.Load(ls),
		bm: bytematcher.Load(ls),
		ids: func(li *persist.LoadSaver) []core.Identifier {
			ids := make([]core.Identifier, li.LoadTinyUInt())
			for i := range ids {
				ids[i] = core.LoadIdentifier(li)
			}
			return ids
		}(ls),
		buffers: siegreader.New(),
	}, ls.Err
}

// String representation of a Siegfried struct
func (s *Siegfried) String() string {
	str := "IDENTIFIERS\n"
	for _, i := range s.ids {
		str += i.String()
	}
	str += "\nEXTENSION MATCHER\n"
	str += s.em.String()
	str += "\nCONTAINER MATCHER\n"
	str += s.cm.String()
	str += "\nBYTE MATCHER\n"
	str += s.bm.String()
	return str
}

// Yaml representation of a Siegfried struct.
// This is the provenace block at the beginning of sf results and includes Yaml descriptions for each identifier.
func (s *Siegfried) Yaml() string {
	version := config.Version()
	str := fmt.Sprintf(
		"---\nsiegfried   : %d.%d.%d\nscandate    : %v\npersist   : %s\ncreated     : %v\nidentifiers : \n",
		version[0], version[1], version[2],
		time.Now().Format(time.RFC3339),
		config.SignatureBase(),
		s.C.Format(time.RFC3339))
	for _, id := range s.ids {
		d := id.Describe()
		str += fmt.Sprintf("  - name    : '%v'\n    details : '%v'\n", d[0], d[1])
	}
	return str
}

// JSON representation of a Siegfried struct.
// This is the provenace block at the beginning of sf results and includes descriptions for each identifier.
func (s *Siegfried) Json() string {
	version := config.Version()
	str := fmt.Sprintf(
		"{\"siegfried\":\"%d.%d.%d\",\"scandate\":\"%v\",\"persist\":\"%s\",\"created\":\"%v\",\"identifiers\":[",
		version[0], version[1], version[2],
		time.Now().Format(time.RFC3339),
		config.SignatureBase(),
		s.C.Format(time.RFC3339))
	for i, id := range s.ids {
		if i > 0 {
			str += ","
		}
		d := id.Describe()
		str += fmt.Sprintf("{\"name\":\"%s\",\"details\":\"%s\"}", d[0], d[1])
	}
	str += "],"
	return str
}

// Identify identifies a stream or file object.
// It takes the name of the file/stream (if unknown, give an empty string) and an io.Reader
// It returns a channel of identifications and an error
func (s *Siegfried) Identify(n string, r io.Reader) (chan core.Identification, error) {
	buffer, err := s.buffers.Get(r)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("siegfried: error reading file; got %v", err)
	}
	res := make(chan core.Identification)
	recs := make([]core.Recorder, len(s.ids))
	for i, v := range s.ids {
		recs[i] = v.Recorder()
	}
	// Extension Matcher
	if len(n) > 0 {
		ems, _ := s.em.Identify(n, nil) // we don't care about an error here
		for v := range ems {
			for _, rec := range recs {
				if rec.Record(core.ExtensionMatcher, v) {
					break
				}
			}
		}
	}
	// Container Matcher
	if s.cm != nil {
		cms, cerr := s.cm.Identify(n, buffer)
		for v := range cms {
			for _, rec := range recs {
				if rec.Record(core.ContainerMatcher, v) {
					break
				}
			}
		}
		err = cerr
	}
	satisfied := true
	for _, rec := range recs {
		if !rec.Satisfied() {
			satisfied = false
		}
	}
	// Byte Matcher
	if !satisfied {
		ids, _ := s.bm.Identify("", buffer) // we don't care about an error here
		for v := range ids {
			for _, rec := range recs {
				if rec.Record(core.ByteMatcher, v) {
					break
				}
			}
		}
	}
	s.buffers.Put(buffer)
	go func() {
		for _, rec := range recs {
			rec.Report(res)
		}
		close(res)
	}()
	return res, err
}

// InspectTestTree checks with the byte matcher to see what identification results subscribe to a particular test
// tree index. It can be used when identifying in a debug mode to check which identification results trigger
// which strikes
func (s *Siegfried) InspectTestTree(tti int) string {
	bm := s.bm.(*bytematcher.Matcher)
	idxs := bm.InspectTestTree(tti)
	if idxs == nil {
		return "No test tree at this index"
	}
	res := make([]string, len(idxs))
	for i, v := range idxs {
		for _, id := range s.ids {
			ok, str := id.Recognise(core.ByteMatcher, v)
			if ok {
				res[i] = str
				break
			}
		}
	}
	return "Test tree indexes match:\n" + strings.Join(res, "\n")
}

// Buffer returns the last buffer inspected
// The purpose is to prevent unnecessary double-up of IO e.g. when unzipping files post-identification
func (s *Siegfried) Buffer() siegreader.Buffer {
	last := s.buffers.Last()
	last.SetQuit(make(chan struct{})) // may have already closed the quit channel
	return last
}

// Update checks whether a Siegfried struct is due for update, by testing whether the time given is after the time
// the signature was created
func (s *Siegfried) Update(t string) bool {
	tm, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return false
	}
	return tm.After(s.C)
}
