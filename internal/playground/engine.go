package playground

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"

	"k3l.io/go-eigentrust/pkg/basic"
	"k3l.io/go-eigentrust/pkg/sparse"

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
		localTrust  *sparse.Matrix
		preTrust    *sparse.Vector
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
	ltDim, err := localTrust.Dim()
	if err != nil {
		return err
	}
	if hasPeerNames {
		// peer name files are the authoritative source of dimension
		n := len(peerNames)
		switch {
		case ltDim < n:
			basic.GrowLocalTrust(localTrust, n-ltDim)
		case ltDim > n:
			panic("localTrust is larger than peerNames")
		}
		switch {
		case preTrust.Dim < n:
			basic.GrowTrustVector(preTrust, n-preTrust.Dim)
		case preTrust.Dim > n:
			panic("preTrust is larger than peerNames")
		}
	} else {
		// grow the smaller one
		switch {
		case ltDim < preTrust.Dim:
			basic.GrowLocalTrust(localTrust, preTrust.Dim-ltDim)
		case preTrust.Dim < ltDim:
			basic.GrowTrustVector(preTrust, ltDim-preTrust.Dim)
		}
	}
	dim := preTrust.Dim
	preTrusted := make([]bool, len(preTrust.Entries))
	for _, e := range preTrust.Entries {
		preTrusted[e.Index] = true
	}
	numLocalTrusts := 0
	for _, row := range localTrust.Entries {
		numLocalTrusts += len(row)
	}
	localTrustDensity := float64(numLocalTrusts) / float64(preTrust.Dim) / float64(preTrust.Dim)

	basic.CanonicalizeTrustVector(preTrust)
	err = basic.CanonicalizeLocalTrust(localTrust, preTrust)
	if err != nil {
		return errors.Wrap(err, "cannot canonicalize local trust")
	}
	trustScores, err := basic.Compute(gc.Request.Context(),
		localTrust, preTrust, float64(hunchPercent)/100.0, 1e-15, nil, nil)
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

	var entries []Entry
	for i := 0; i < dim; i++ {
		entries = append(entries,
			Entry{
				Index:    i,
				Name:     getPeerName(i),
				Score:    0,
				ScoreLog: math.Inf(-1),
			})
	}
	for _, e := range trustScores.Entries {
		entries[e.Index].Score = e.Value
		entries[e.Index].ScoreLog = math.Log10(e.Value)
	}
	sort.Sort(ByScore(entries))
	peerNamesFileName := ""
	if hasPeerNames {
		peerNamesFileName = peerNamesFile.Filename
	}
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
