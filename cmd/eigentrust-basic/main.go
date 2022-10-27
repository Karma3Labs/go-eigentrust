package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"

	"log"

	"github.com/gin-gonic/gin"
	"gonum.org/v1/gonum/mat"
)

func renderErrorPage(gc *gin.Context, format string, args ...any) {
	gc.HTML(http.StatusBadRequest, "error.html",
		gin.H{"message": fmt.Sprintf(format, args...)})
}

func sum(v []float64) float64 {
	var s, c float64
	for _, v1 := range v {
		y := v1 - c
		t := s + y
		c = (t - s) - y
		s = t
	}
	return s
}

type Entry struct {
	Index    int
	Name     string
	Score    float64
	ScoreLog float64
}

type ByScore []Entry

func (a ByScore) Len() int           { return len(a) }
func (a ByScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByScore) Less(i, j int) bool { return a[i].Score > a[j].Score }

func main() {
	gr := gin.Default()
	gr.LoadHTMLGlob("templates/*")
	gr.GET("/", func(gc *gin.Context) {
		gc.HTML(http.StatusOK, "index.html", gin.H{})
	})
	gr.POST("/calculate", func(gc *gin.Context) {
		peerNamesFile, err := gc.FormFile("peerNamesFile")
		if err == http.ErrMissingFile {
			peerNamesFile = nil
		} else if err != nil {
			renderErrorPage(gc, "cannot read peer names file: %s", err)
			return
		}
		localTrustFile, err := gc.FormFile("localTrustFile")
		if err != nil {
			renderErrorPage(gc, "cannot read local trust file: %s", err)
			return
		}
		preTrustFile, err := gc.FormFile("preTrustFile")
		if err != nil {
			renderErrorPage(gc, "cannot read personal trust file: %s", err)
			return
		}
		hunchPercentStr := gc.DefaultPostForm("hunchPercent", "10")
		hunchPercent, err := strconv.Atoi(hunchPercentStr)
		if err != nil {
			renderErrorPage(gc, "invalid hunch percent %#v: %s",
				hunchPercentStr, err)
			return
		}
		if hunchPercent < 0 || hunchPercent > 100 {
			renderErrorPage(gc, "hunch percent %#v out of range [0..100]",
				hunchPercent)
			return
		}
		_ = gc.DefaultPostForm("omitPreTrusted", "off") == "on"
		var peerNames []string
		var fields []string
		peerIndices := map[string]int{}
		if peerNamesFile != nil {
			f, err := peerNamesFile.Open()
			if err != nil {
				renderErrorPage(gc, "cannot open peer names file: %s", err)
				return
			}
			defer f.Close()
			reader := csv.NewReader(f)
			for fields, err = reader.Read(); err == nil; fields, err = reader.Read() {
				if len(fields) != 1 {
					renderErrorPage(gc,
						"extra fields in peer names file (line %d, fields are %#v)",
						len(peerNames)+1, fields)
					return
				}
				name := fields[0]
				if _, ok := peerIndices[name]; ok {
					renderErrorPage(gc,
						"duplicate peer name %s", name)
					return
				}
				peerIndices[name] = len(peerNames)
				peerNames = append(peerNames, name)
			}
			if err != io.EOF {
				renderErrorPage(gc, "cannot read peer names file: %s", err)
				return
			}
		}
		s := &mat.Dense{}
		numLocalTrusts := 0
		{
			f, err := localTrustFile.Open()
			if err != nil {
				renderErrorPage(gc, "cannot open local trust file: %s", err)
				return
			}
			defer func() { _ = f.Close() }()
			reader := csv.NewReader(f)
			for fields, err = reader.Read(); err == nil; fields, err = reader.Read() {
				// TODO: check fields
				from := fields[0]
				to := fields[1]
				var level float64 = 1.0
				if len(fields) >= 3 {
					if l, err := strconv.ParseFloat(fields[2], 64); err != nil {
						renderErrorPage(gc,
							"invalid local trust level literal %#v: %s",
							fields[2], err)
						return
					} else if l < 0 {
						renderErrorPage(gc, "negative local trust level %f", l)
						return
					} else {
						level = l
					}
				}
				var fromIndex, toIndex int
				if peerNamesFile != nil {
					var found bool
					if fromIndex, found = peerIndices[from]; !found {
						renderErrorPage(gc, "unknown peer name %#v", from)
						return
					}
					if toIndex, found = peerIndices[to]; !found {
						renderErrorPage(gc, "unknown peer name %#v", to)
						return
					}
				} else {
					if fromIndex, err = strconv.Atoi(from); err != nil {
						renderErrorPage(gc, "invalid peer index literal %#v",
							from)
						return
					}
					if toIndex, err = strconv.Atoi(to); err != nil {
						renderErrorPage(gc, "invalid peer index literal %#v",
							to)
						return
					}
					if fromIndex < 0 {
						renderErrorPage(gc, "negative peer index %#v",
							fromIndex)
						return
					}
					if toIndex < 0 {
						renderErrorPage(gc, "negative peer index %#v", toIndex)
						return
					}
				}
				n := fromIndex
				if n < toIndex {
					n = toIndex
				}
				n++
				if nr, nc := s.Dims(); nr < n {
					s = s.Grow(n-nr, n-nc).(*mat.Dense)
				}
				if level > 0 {
					s.Set(fromIndex, toIndex, level)
					numLocalTrusts++
				}
			}
			if err != io.EOF {
				renderErrorPage(gc, "cannot read local trust file: %s", err)
				return
			}
		}
		var pv []float64
		{
			f, err := preTrustFile.Open()
			if err != nil {
				renderErrorPage(gc, "cannot open personal trust file: %s", err)
				return
			}
			defer func() { _ = f.Close() }()
			reader := csv.NewReader(f)
			for fields, err = reader.Read(); err == nil; fields, err = reader.Read() {
				peerName := fields[0]
				var level float64 = 1.0
				if len(fields) >= 2 {
					if l, err := strconv.ParseFloat(fields[1], 64); err != nil {
						renderErrorPage(gc,
							"invalid personal trust level literal %#v: %s",
							fields[1], err)
						return
					} else if l < 0 {
						renderErrorPage(gc, "negative personal trust level %f",
							l)
						return
					} else {
						level = l
					}
				}
				var peerIndex int
				if peerNamesFile != nil {
					var ok bool
					if peerIndex, ok = peerIndices[peerName]; !ok {
						renderErrorPage(gc,
							"unknown personally trusted peer %s",
							peerName)
						return
					}
				} else {
					if peerIndex, err = strconv.Atoi(peerName); err != nil {
						renderErrorPage(gc,
							"invalid personally trusted peer index literal %#v: %s",
							peerName, err)
						return
					}
					if peerIndex < 0 {
						renderErrorPage(gc,
							"negative personally trusted peer index %#v",
							peerIndex)
						return
					}
				}
				for len(pv) <= peerIndex {
					pv = append(pv, 0)
				}
				pv[peerIndex] = level
			}
			if err != io.EOF {
				renderErrorPage(gc, "cannot read personal trust file: %s", err)
				return
			}
		}
		nr, nc := s.Dims()
		if nr != nc {
			panic(fmt.Sprintf("nr=%d != nc=%d", nr, nc))
		}
		for len(pv) < nr {
			pv = append(pv, 0)
		}
		// Normalize pv
		pSum := sum(pv)
		if pSum == 0 {
			renderErrorPage(gc, "no personal trust")
			return
		}
		for i, v := range pv {
			pv[i] = v / pSum
		}
		c := mat.NewDense(nr, nc, nil)
		for i := 0; i < nr; i++ {
			row := s.RawRowView(i)
			rowSum := sum(row)
			if rowSum == 0 {
				for j := 0; j < nc; j++ {
					c.Set(i, j, pv[j])
				}
			} else {
				for j := 0; j < nc; j++ {
					c.Set(i, j, s.At(i, j)/rowSum)
				}
			}
		}
		p := mat.NewVecDense(len(pv), pv)
		a := float64(hunchPercent) / 100.0
		e := 1e-15
		t := mat.VecDenseCopyOf(p)
		ct := c.T()
		ap := &mat.VecDense{}
		ap.ScaleVec(a, p)
		d := e
		iter := 0
		for d >= e {
			iter++
			t0 := mat.VecDenseCopyOf(t)
			t.MulVec(ct, t)
			t.AddScaledVec(ap, 1-a, t)
			t0.SubVec(t, t0)
			d = t0.Norm(2)
			log.Printf("iteration %d, norm %#v\n", iter, d)
			if iter >= 1000 {
				renderErrorPage(gc,
					"trust vector did not converge after %d iterations; see server log",
					iter)
				return
			}
		}
		getPeerName := func(i int) string {
			if peerNamesFile != nil {
				return peerNames[i]
			}
			return fmt.Sprintf("Peer %d", i)
		}
		var entries []Entry
		for i := 0; i < nr; i++ {
			score := t.AtVec(i)
			entries = append(entries,
				Entry{
					Index:    i,
					Name:     getPeerName(i),
					Score:    score,
					ScoreLog: math.Log10(score),
				})
		}
		sort.Sort(ByScore(entries))
		preTrusted := make([]bool, nr)
		for i, v := range pv {
			if v > 0 {
				preTrusted[i] = true
			}
		}
		peerNamesFileName := ""
		if peerNamesFile != nil {
			peerNamesFileName = peerNamesFile.Filename
		}
		localTrustDensity := float64(numLocalTrusts) / float64(nr) / float64(nr)
		gc.HTML(http.StatusOK, "result.html",
			gin.H{
				"PeerNamesFileName":        peerNamesFileName,
				"LocalTrustFileName":       localTrustFile.Filename,
				"NumLocalTrusts":           numLocalTrusts,
				"LocalTrustDensityPercent": localTrustDensity * 100,
				"PreTrustFileName":         preTrustFile.Filename,
				"HunchPercent":             hunchPercent,
				"PreTrusted":               preTrusted,
				"Entries":                  entries,
			})
	})
	gr.Run()
}
