package main

import (
	"log"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"bytes"
	"encoding/json"
	"time"
	"os/exec"
	"fmt"
)

func main() {
	flag.Parse()

	objectidtable, err	:= parseSnapshotList()
	if err != nil {
		log.Fatalf("can't get kopia infos: %v", err)
	}

	for _, fn := range flag.Args() {
		log.Println("arg", fn)
		fileinfo, err := os.Lstat(fn)
		if err != nil {
			// This is probably bad.
			log.Fatalf("can't lstat %q: %v", fn, err)
		}


		if fileinfo.Mode() & os.ModeSymlink == 0 {
			continue
		}
		
		log.Println(fn,  "is a symlink")

		symlinkpath, err := os.Readlink(fn)
		if err != nil {
			log.Fatalf("can't get where the symlink %q points: %v", fn, err)
		}

		log.Println("symlinkpath value", symlinkpath)

		// TODO(rjk): here, I have to figure out the kopia target.
		// mv the symlink to a different (backup) path

		// we get a hash table of objectids, hashed by path

		// Compute the restore destination --------
		absdest, err := filepath.Abs(fn)
		if err != nil {
			log.Fatalf("can't abs %q: %v", fn, err)
		}
		
		log.Println("absdest", absdest)

		components := strings.Split(absdest, string(filepath.Separator))


		if len(components) > 2 && components[0] == "" && components[1] == "private" && components[2] == "tmp" {
			// Remove "private" from the components.
			newc := append([]string{}, components[0])
			newc = append(newc, components[2:]...)
			components = newc
		}
		log.Printf("%#v", components)

		// find the object id: for each path by seeing if we have a match in the hash table for
		// a prefix of components
		objid, srccomponents := objectidtable.getObjectId(components)
		log.Printf("%q, %#v", objid, srccomponents)
		if objid == "" {
			log.Fatalf("can't find objectid for %q", fn)
		}

		srcrelpath := filepath.Dir(filepath.Join(srccomponents...))
		
		// Compute the restore source
		restoresrc := objid + string(filepath.Separator) + filepath.Clean(filepath.Join(srcrelpath, symlinkpath))
		log.Println("restoresrc", restoresrc)

		// TODO(rjk): This is unnecessary and can be removed.
		// Compute the restore target
		restoretarget :=  filepath.Dir( filepath.Join(components...)) 
			if components[0] == "" {
				restoretarget = "/" + restoretarget
			}
		log.Println("restoretarget", restoretarget)

		// remove the link (kopia will do this)
	kopia := exec.Command("/usr/local/bin/kopia", "restore", restoresrc, absdest)
	spew, err := kopia.CombinedOutput()
	if err != nil {
		log.Fatal("annexrestore can't run kopia", err, "spew:", string(spew))
	}
	log.Println("Finished running kopia without errors, spew discarded")

	}
}

type ObjectIdPath map[string]string

func (objectidtable ObjectIdPath) getObjectId(components []string) (string, []string) {
		for i := 1; i < len(components); i++ {
			path :=   filepath.Join(components[0:i]...)
			if components[0] == "" {
				path = "/" + path
			}

			log.Println("objectid", i, path)
			if id, ok := objectidtable[path]; ok {
				return id, components[i:]
			}
		}
	
	// TODO(rjk): error handling
	return "", []string{}
}

type KopiaSnapshot struct {
	Id string `json:"id"`
	Source KopiaSource `json:"source"`
	RawStartTime string `json:"startTime"`
	RawEndTime string `json:"endTime"`
	endtime time.Time
	RootEntry KopiaRootEntry `json:"rootEntry"`
}

type KopiaSource struct {
	Host string `json:"host"`
	UserName string `json:"userName"`
	Path string `json:"path"`
}

type KopiaRootEntry struct {
	Obj string `json:"obj"`
	Summary KopiaSummary `json:"summ"`
}

type KopiaSummary struct {
	NumFailed int  `json:"numFailed"`
}

const timeformat = "2006-01-02T15:04:05.9999-07:00"

func parseSnapshotList() (ObjectIdPath, error) {
	kopia := exec.Command("/usr/local/bin/kopia", "snapshot", "list", "--json")
	spew, err := kopia.CombinedOutput()
	if err != nil {
		log.Println("can't run kopia snapshot list", err, "spew:", string(spew))
		return ObjectIdPath{}, fmt.Errorf("can't run kopia snapshot list: %v, %s", err, "spew:", string(spew))
	}
	log.Println("Finished running kopia snapshot list without errors, parsing spew")

	buffy := bytes.NewBuffer(spew)
	decoder := json.NewDecoder(buffy)
	var snapshotlist []KopiaSnapshot

	if err := decoder.Decode(&snapshotlist); err != nil {
		log.Fatal("can't decode json spew", err)
	}
	
	// That's a lot of spew.
	// log.Println(snapshotlist)

	currenthost, err := os.Hostname()
	if err != nil {
		log.Fatal("Hostname:", err)
	}
	// log.Println("hostname from os.Hostname", currenthost)

	pathhash := make(map[string]KopiaSnapshot)
	for _, sr := range snapshotlist {
		if sr.Source.Host != currenthost {
			continue
		}

		// Prune out the failed cases
		if sr.RootEntry.Summary.NumFailed > 0 {
			log.Println(sr.RootEntry.Obj, "failed, skipping")
			continue
		}

		// Parse the time.
		tm, err := time.Parse(timeformat, sr.RawEndTime)
		if err != nil {
			log.Fatal("can't parse time %s: %v", sr.RawEndTime, err)
		}
		sr.endtime = tm


		path := sr.Source.Path

		if _, ok := pathhash[path]; !ok {
			pathhash[path] = sr
			continue
		}

		cr := pathhash[path]

		// if sr newer than cr
		if cr.endtime.Before(sr.endtime) {
			pathhash[path] = sr
		}
	}

	for k, v := range pathhash {
		log.Printf("%q: %#v", k, v)
	}

	// Reprocess into the desired hashtable
	objidpath := make(ObjectIdPath, len(pathhash))
	for k, v := range pathhash {
		objidpath[k] = v.RootEntry.Obj
	}

	log.Println(objidpath)
	return objidpath, nil
}
