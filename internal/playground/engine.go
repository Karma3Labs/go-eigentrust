package playground

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"

	"k3l.io/go-eigentrust/pkg/basic"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

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

func AddRoutes(routes gin.IRoutes) {
	routes.GET("/", handle(index))
	routes.POST("/calculate", handle(calculate))
}

func handle(f func(gc *gin.Context) error) func(*gin.Context) {
	return func(gc *gin.Context) {
		if err := f(gc); err != nil {
			gc.HTML(http.StatusBadRequest, "error.html",
				gin.H{"message": err.Error()})
		}
	}
}

func index(gc *gin.Context) error {
	gc.HTML(http.StatusOK, "index.html", gin.H{})
	return nil
}

func calculate(gc *gin.Context) error {
	var hasPeerNames bool
	peerNamesFile, err := gc.FormFile("peerNamesFile")
	switch err {
	case http.ErrMissingFile:
		hasPeerNames = false
	case nil:
		hasPeerNames = true
	default:
		return errors.Wrapf(err, "cannot read peerNamesFile input")
	}
	localTrustFile, err := gc.FormFile("localTrustFile")
	if err != nil {
		return errors.Wrapf(err, "cannot read localTrustFile input")
	}
	preTrustFile, err := gc.FormFile("preTrustFile")
	if err != nil {
		return errors.Wrapf(err, "cannot read preTrustFile input")
	}
	hunchPercentStr := gc.DefaultPostForm("hunchPercent", "10")
	hunchPercent, err := strconv.Atoi(hunchPercentStr)
	if err != nil {
		return errors.Wrapf(err, "invalid hunch percent %#v", hunchPercentStr)
	} else if hunchPercent < 0 || hunchPercent > 100 {
		return errors.Errorf("hunch percent %#v out of range [0..100]",
			hunchPercent)
	}
	var (
		peerNames   []string
		peerIndices map[string]int
		localTrust  basic.LocalTrust
		preTrust    basic.TrustVector
	)
	if hasPeerNames {
		f, err := peerNamesFile.Open()
		if err != nil {
			return errors.Wrap(err, "cannot open peer names file")
		}
		defer func() { _ = f.Close() }()
		peerNames, peerIndices, err = basic.ReadPeerNamesFromCsv(csv.NewReader(f))
		if err != nil {
			return errors.Wrap(err, "cannot read peer names file")
		}
	}
	{
		f, err := localTrustFile.Open()
		if err != nil {
			return errors.Wrap(err, "cannot open local trust file")
		}
		defer func() { _ = f.Close() }()
		localTrust, err = basic.ReadLocalTrustFromCsv(csv.NewReader(f),
			peerIndices)
		if err != nil {
			return errors.Wrap(err, "cannot read local trust file")
		}
	}
	{
		f, err := preTrustFile.Open()
		if err != nil {
			return errors.Wrap(err, "cannot open personal trust file")
		}
		defer func() { _ = f.Close() }()
		preTrust, err = basic.ReadTrustVectorFromCsv(csv.NewReader(f),
			peerIndices)
		if err != nil {
			return errors.Wrap(err, "cannot read personal trust file")
		}
	}
	if hasPeerNames {
		// peer name files are the authoritative source of dimension
		n := len(peerNames)
		switch {
		case localTrust.Dim() < n:
			localTrust = localTrust.Grow(n - localTrust.Dim())
		case localTrust.Dim() > n:
			panic("localTrust is larger than peerNames")
		}
		switch {
		case preTrust.Len() < n:
			preTrust = preTrust.Grow(n - preTrust.Len())
		case preTrust.Len() > n:
			panic("preTrust is larger than peerNames")
		}
	} else {
		// grow the smaller one
		switch {
		case localTrust.Dim() < preTrust.Len():
			localTrust.Grow(preTrust.Len() - localTrust.Dim())
		case preTrust.Len() < localTrust.Dim():
			preTrust.Grow(localTrust.Dim() - preTrust.Len())
		}
	}
	p := preTrust.Canonicalize()
	c, err := localTrust.Canonicalize(p)
	if err != nil {
		return errors.Wrap(err, "cannot canonicalize local trust")
	}
	trustScores, err := basic.Compute(gc.Request.Context(),
		c, p, float64(hunchPercent)/100.0, 1e-15, nil, nil)
	if err != nil {
		return errors.Wrap(err, "cannot compute EigenTrust scores")
	}

	getPeerName := func(i int) string {
		if hasPeerNames {
			return peerNames[i]
		} else {
			return fmt.Sprintf("Peer %d", i)
		}
	}

	nr, nc := localTrust.Dims()
	var entries []Entry
	for i := 0; i < nr; i++ {
		score := trustScores.AtVec(i)
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
	np := preTrust.Len()
	for i := 0; i < np; i++ {
		if preTrust.AtVec(i) > 0 {
			preTrusted[i] = true
		}
	}
	peerNamesFileName := ""
	if hasPeerNames {
		peerNamesFileName = peerNamesFile.Filename
	}
	numLocalTrusts := 0
	for i := 0; i < nr; i++ {
		for j := 0; j < nc; j++ {
			if localTrust.At(i, j) > 0 {
				numLocalTrusts++
			}
		}
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
	return nil
}
