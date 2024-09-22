package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k3l.io/go-eigentrust/pkg/api/openapi"
	"k3l.io/go-eigentrust/pkg/basic"
	"k3l.io/go-eigentrust/pkg/peer"
	"k3l.io/go-eigentrust/pkg/sparse"
	spopt "k3l.io/go-eigentrust/pkg/sparse/option"
	"k3l.io/go-eigentrust/pkg/util"
)

var (
	// basicComputeCmd represents the compute command
	basicComputeCmd = &cobra.Command{
		Use:   "compute",
		Short: "Submit a basic EigenTrust compute request.",
		Long:  `Submit a basic EigenTrust compute request.`,
		Args:  cobra.MatchAll(cobra.NoArgs),
		Run:   runBasicCompute,
	}
	localTrustURI         string
	preTrustURI           string
	initialTrustURI       string
	alpha                 float64
	epsilon               float64
	flatTail              int
	numLeaders            int
	outputFilename        string
	flatTailStatsFilename string
	maxIterations         int
	minIterations         int
	checkFreq             int
	csvHasHeader          bool
	rawPeerIds            bool
	peerMap               *peer.Map
	printRequest          bool
)

func pathIntoFileRef(path string, ref *openapi.TrustRef) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	err = ref.FromObjectStorageTrustRef(openapi.ObjectStorageTrustRef{Url: "file://" + path})
	if err != nil {
		return err
	}
	ref.Scheme = openapi.Objectstorage
	return nil
}

func trustMatrixURIToRef(uri string, ref *openapi.TrustRef) error {
	parsed, err := url.Parse(uri)
	if err != nil {
		return err
	}
	switch parsed.Scheme {
	case "file", "":
		path := parsed.Path
		if path == "" {
			path = parsed.Opaque
		}
		if useFileURI {
			return pathIntoFileRef(path, ref)
		}
		return loadInlineTrustMatrix(path, ref)
	default:
		return fmt.Errorf("invalid local trust URI scheme %#v", parsed.Scheme)
	}
}

func loadInlineTrustMatrix(filename string, ref *openapi.TrustRef) error {
	logger.Trace().Str("filename", filename).Msg("loading inline local trust")
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return loadInlineTrustMatrixCSV(filename, ref)
	default:
		return fmt.Errorf("invalid local trust file type %#v", ext)
	}
}

func loadInlineTrustMatrixCSV(
	filename string, ref *openapi.TrustRef,
) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer util.Close(f)
	reader := csv.NewReader(f)
	ctx := context.TODO()
	peerMapOption := spopt.LiteralIndices
	if peerMap != nil {
		peerMapOption = spopt.IndicesInto(peerMap)
	}
	m, err := sparse.NewCSRMatrixFromCSV(ctx, reader, peerMapOption)
	if err != nil {
		return err
	}
	inline, err := openapi.InlineFromMatrix(ctx, m)
	if err != nil {
		return err
	}
	err = ref.FromInlineTrustRef(*inline)
	if err != nil {
		return fmt.Errorf("cannot wrap inline trust matrix: %w", err)
	}
	ref.Scheme = openapi.Inline
	return nil
}

func trustVectorURIToRef(uri string, ref *openapi.TrustRef) error {
	parsed, err := url.Parse(uri)
	if err != nil {
		return err
	}
	switch parsed.Scheme {
	case "file", "":
		path := parsed.Path
		if path == "" {
			path = parsed.Opaque
		}
		if useFileURI {
			return pathIntoFileRef(path, ref)
		}
		return loadInlineTrustVector(path, ref)
	default:
		return fmt.Errorf("invalid trust vector URI scheme %#v", parsed.Scheme)
	}
}

func loadInlineTrustVector(filename string, ref *openapi.TrustRef) error {
	logger.Trace().Str("filename", filename).Msg("loading inline trust vector")
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return loadInlineTrustVectorFromCSV(filename, ref)
	default:
		return fmt.Errorf("invalid trust vector file type %#v", ext)
	}
}

func loadInlineTrustVectorFromCSV(
	filename string, ref *openapi.TrustRef,
) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer util.Close(f)
	reader := csv.NewReader(f)
	ctx := context.TODO()
	peerMapOption := spopt.LiteralIndices
	if peerMap != nil {
		peerMapOption = spopt.IndicesInto(peerMap)
	}
	v, err := sparse.NewVectorFromCSV(ctx, reader, peerMapOption)
	if err != nil {
		return err
	}
	inline, err := openapi.InlineFromVector(ctx, v)
	if err != nil {
		return err
	}
	if err = ref.FromInlineTrustRef(*inline); err != nil {
		return fmt.Errorf("cannot wrap inline trust vector: %w", err)
	}
	ref.Scheme = openapi.Inline
	return nil
}

func writeInlineTrustVectorIntoCSV(
	ctx context.Context, itv *openapi.InlineTrustRef, filename string,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	v, err := openapi.VectorFromInline(ctx, itv, spopt.IncludeZero,
		spopt.AllowNegative)
	if err != nil {
		return fmt.Errorf("cannot load inline trust vector: %w", err)
	}
	file, err := util.OpenOutputFile(filename)
	if err != nil {
		return fmt.Errorf("cannot open output file: %w", err)
	}
	defer util.Close(file)
	w := csv.NewWriter(file)
	indexOption := spopt.LiteralIndices
	if peerMap != nil {
		indexOption = spopt.IndicesIn(peerMap)
	}
	err = v.WriteIntoCSV(ctx, w, spopt.IncludeZero, indexOption)
	if err != nil {
		return fmt.Errorf("cannot write into CSV: %w", err)
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return fmt.Errorf("cannot flush CSV writes: %w", err)
	}
	return nil
}

func writeFlatTailStats(stats basic.FlatTailStats, filename string) error {
	file, err := util.OpenOutputFile(filename)
	if err != nil {
		return fmt.Errorf("cannot open flat-tail stats file for writing: %w",
			err)
	}
	defer util.Close(file)
	jsonEncoder := json.NewEncoder(file)
	if err := jsonEncoder.Encode(stats); err != nil {
		return err
	}
	return nil
}

func runBasicCompute( /*cmd*/ *cobra.Command /*args*/, []string) {
	basicSetupEndpoint()
	var err error
	if useFileURI {
		rawPeerIds = true
	}
	if !rawPeerIds {
		peerMap = peer.NewMap()
	}
	client, err := openapi.NewClientWithResponses(endpoint)
	if err != nil {
		logger.Err(err).Msg("cannot create an API client")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	epsilonP := &epsilon
	if epsilon == 0 {
		epsilonP = nil
	}
	requestBody := openapi.ComputeWithStatsJSONRequestBody{
		Alpha:        &alpha,
		Epsilon:      epsilonP,
		PreTrust:     nil,
		InitialTrust: nil,
	}
	err = trustMatrixURIToRef(localTrustURI, &requestBody.LocalTrust)
	if err != nil {
		logger.Err(err).Msg("cannot parse/load local trust reference")
		return
	}
	if preTrustURI != "" {
		var preTrustRef openapi.TrustRef
		err = trustVectorURIToRef(preTrustURI, &preTrustRef)
		if err != nil {
			logger.Err(err).Msg("cannot parse/load pre-trust reference")
			return
		}
		requestBody.PreTrust = &preTrustRef
	}
	if initialTrustURI != "" {
		var initialTrustRef openapi.TrustRef
		err = trustVectorURIToRef(initialTrustURI, &initialTrustRef)
		if err != nil {
			logger.Err(err).Msg("cannot parse/load initial trust reference")
			return
		}
		requestBody.InitialTrust = &initialTrustRef
	}
	requestBody.FlatTail = &flatTail
	requestBody.NumLeaders = &numLeaders
	if maxIterations > 0 {
		requestBody.MaxIterations = &maxIterations
	}
	// default minIterations==-1 lets server choose default (same as checkFreq)
	if minIterations >= 0 {
		requestBody.MinIterations = &minIterations
	}
	if checkFreq > 1 {
		requestBody.CheckFreq = &checkFreq
	}
	if printRequest {
		req := struct {
			Body    *openapi.ComputeWithStatsJSONRequestBody `json:"body"`
			PeerIds []string                                 `json:"peerIds"`
		}{&requestBody, peerMap.Ids()}
		err = json.NewEncoder(os.Stdout).Encode(req)
		if err != nil {
			logger.Err(err).Msg("cannot encode/print the request body")
		}
		return
	}
	resp, err := client.ComputeWithStatsWithResponse(ctx, requestBody)
	if err != nil {
		logger.Err(err).Msg("request failed")
		return
	}
	switch resp.StatusCode() {
	case 200:
		if resp.JSON200 == nil {
			logger.Error().Msg("cannot recover HTTP 200 response")
		} else if inlineEigenTrust, err := resp.JSON200.EigenTrust.AsInlineTrustRef(); err != nil {
			logger.Error().Msg("cannot parse response")
		} else {
			if err = writeInlineTrustVectorIntoCSV(
				ctx, &inlineEigenTrust, outputFilename,
			); err != nil {
				logger.Err(err).Msg("cannot write output file")
			}
			if err = writeFlatTailStats(
				resp.JSON200.FlatTailStats, flatTailStatsFilename,
			); err != nil {
				logger.Err(err).Msg("cannot write flat-tail stats file")
			}
		}
	case 400:
		if resp.JSON400 != nil {
			logger.Error().Str("error", resp.JSON400.Message).
				Msg("invalid request")
		}
	default:
		logger.Error().Str("status", resp.HTTPResponse.Status).
			Msg("server returned unknown status code")
	}
}

func init() {
	basicCmd.AddCommand(basicComputeCmd)
	basicComputeCmd.Flags().StringVarP(&localTrustURI, "local-trust", "l",
		"file:localtrust.csv",
		`Local trust reference URI.
file URIs are parsed and transmitted as inline;
schemaless URIs are assumed to be file URIs.`)
	basicComputeCmd.Flags().StringVarP(&preTrustURI, "pre-trust", "p",
		"",
		`Pre-trust reference URI;
file URIs are parsed and transmitted as inline.
If not given, server uses uniform trust vector by default.`)
	basicComputeCmd.Flags().StringVarP(&initialTrustURI, "initial-trust", "i",
		"",
		`Initial trust reference URI;
file URIs are parsed and transmitted as inline.
If not given, server uses pre-trust vector by default.`)
	basicComputeCmd.Flags().Float64VarP(&alpha, "alpha", "a", 0.5,
		`Alpha value, between 0.0 and 1.0 inclusive.
Higher value biases the computation toward pre-trust.`)
	basicComputeCmd.Flags().Float64VarP(&epsilon, "epsilon", "e", 0.0,
		`Epsilon (error max).  0 (default) uses server default.`)
	basicComputeCmd.Flags().IntVar(&flatTail, "flat-tail", 0,
		`Flat-tail threshold length. 0 (default) disables flat-tail algorithm.`)
	basicComputeCmd.Flags().IntVar(&numLeaders, "num-leaders", 0,
		`Number of top-ranking peers (leaders) to consider
for flat-tail algorithm and stats.
0 (default) includes all peers.`)
	basicComputeCmd.Flags().StringVarP(&outputFilename, "output", "o",
		"-",
		`Output file name.
"" suppresses output; "-" (default) uses standard output`)
	basicComputeCmd.Flags().StringVar(&flatTailStatsFilename, "flat-tail-stats",
		"",
		`Flat tail stats output file name.
"" (default) suppresses output; "-" uses standard output`)
	basicComputeCmd.Flags().IntVar(&maxIterations, "max-iterations", 0,
		`Maximum number of iterations. 0 (default) means unlimited`)
	basicComputeCmd.Flags().IntVar(&minIterations, "min-iterations", -1,
		`Minimum number of iterations (default: same as --check-freq)`)
	basicComputeCmd.Flags().IntVar(&checkFreq, "check-freq", 1,
		`Exit criteria check frequency, in number of iterations (default: 1)`)
	basicComputeCmd.Flags().BoolVar(&csvHasHeader, "csv-header", true,
		`Whether input CSV has a header line (default: true)`)
	basicComputeCmd.Flags().BoolVar(&rawPeerIds, "raw-peer-ids", false,
		`Whether to use truster/trustee in input CSV directly as peer indices
(default: false)`)
	basicComputeCmd.Flags().BoolVar(&printRequest, "print-request", false,
		`Print the compute request JSON body and exit`)
	basicComputeCmd.Flags().BoolVarP(&useFileURI, "use-file-uri", "F", false,
		`Use objectstorage scheme with file:// URI for local file;
implies --raw-peer-ids (default: false)`)
}
