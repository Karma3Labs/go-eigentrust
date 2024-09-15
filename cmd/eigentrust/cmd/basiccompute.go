package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"k3l.io/go-eigentrust/pkg/api/openapi"
	"k3l.io/go-eigentrust/pkg/basic"
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
	peerIds               []string
	peerIndices           map[string]int
	printRequest          bool
)

func trustMatrixURIToRef(uri string, ref *openapi.TrustMatrixRef) error {
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
			if path, err := filepath.Abs(path); err != nil {
				return err
			} else if err := ref.FromObjectStorageTrustMatrix(openapi.ObjectStorageTrustMatrix{
				Scheme: openapi.ObjectStorageTrustMatrixSchemeObjectstorage,
				Url:    "file://" + path,
			}); err != nil {
				return err
			}
			ref.Scheme = "objectstorage" // XXX
			return nil
		}
		return loadInlineTrustMatrix(path, ref)
	default:
		return fmt.Errorf("invalid local trust URI scheme %#v", parsed.Scheme)
	}
}

func loadInlineTrustMatrix(filename string, ref *openapi.TrustMatrixRef) error {
	logger.Trace().Str("filename", filename).Msg("loading inline local trust")
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return loadInlineTrustMatrixCsv(filename, ref)
	default:
		return fmt.Errorf("invalid local trust file type %#v", ext)
	}
}

func loadInlineTrustMatrixCsv(
	filename string, ref *openapi.TrustMatrixRef,
) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer util.Close(f)
	reader := csv.NewReader(f)
	inputErrorf := func(field int, format string, v ...interface{}) error {
		line, column := reader.FieldPos(0)
		return fmt.Errorf("%s:%d:%d: %s",
			filename, line, column, fmt.Sprintf(format, v...))
	}
	inputWrapf := func(
		err error, field int, format string, v ...interface{},
	) error {
		line, column := reader.FieldPos(0)
		return fmt.Errorf("%s:%d:%d: %s: %s",
			filename, line, column, fmt.Sprintf(format, v...), err)
	}
	inline := openapi.InlineTrustMatrix{Scheme: openapi.InlineTrustMatrixSchemeInline}
	ignoreFirst := csvHasHeader
	fields, err := reader.Read()
	for ; err == nil; fields, err = reader.Read() {
		if len(fields) < 2 {
			return inputErrorf(0, "too few (%d) fields", len(fields))
		}
		if len(fields) > 3 {
			return inputErrorf(0, "too many (%d) fields", len(fields))
		}
		if ignoreFirst {
			ignoreFirst = false
			continue
		}
		var (
			from, to int
			value    float64
		)
		from, err = getPeerIndex(fields[0])
		switch {
		case err != nil:
			return inputWrapf(err, 0, "invalid from=%#v", fields[0])
		case from < 0:
			return inputErrorf(0, "negative from=%#v", from)
		}
		to, err = getPeerIndex(fields[1])
		switch {
		case err != nil:
			return inputWrapf(err, 1, "invalid to=%#v", fields[1])
		case to < 0:
			return inputErrorf(1, "negative to=%#v", to)
		}
		value, err = strconv.ParseFloat(fields[2], 64)
		switch {
		case err != nil:
			return inputWrapf(err, 2, "invalid trust value=%#v", fields[2])
		}
		inline.Entries = append(inline.Entries,
			openapi.InlineTrustMatrixEntry{I: from, J: to, V: value})
		if inline.Size <= from {
			inline.Size = from + 1
		}
		if inline.Size <= to {
			inline.Size = to + 1
		}
	}
	if inline.Size == 0 {
		return errors.New("empty trust matrix")
	}
	if err != io.EOF {
		return fmt.Errorf("cannot read trust matrix CSV %#v: %w", filename, err)
	}
	if err = ref.FromInlineTrustMatrix(inline); err != nil {
		return fmt.Errorf("cannot wrap inline trust matrix: %w", err)
	}
	ref.Scheme = openapi.TrustMatrixRefSchemeInline
	return nil
}

func trustVectorURIToRef(uri string, ref *openapi.TrustVectorRef) error {
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
			if path, err := filepath.Abs(path); err != nil {
				return err
			} else if err := ref.FromObjectStorageTrustVector(openapi.ObjectStorageTrustVector{
				Scheme: openapi.ObjectStorageTrustVectorSchemeObjectstorage,
				Url:    "file://" + path,
			}); err != nil {
				return err
			}
			ref.Scheme = "objectstorage" // XXX
			return nil
		}
		return loadInlineTrustVector(path, ref)
	default:
		return fmt.Errorf("invalid trust vector URI scheme %#v", parsed.Scheme)
	}
}

func loadInlineTrustVector(filename string, ref *openapi.TrustVectorRef) error {
	logger.Trace().Str("filename", filename).Msg("loading inline trust vector")
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return loadInlineTrustVectorCsv(filename, ref)
	default:
		return fmt.Errorf("invalid trust vector file type %#v", ext)
	}
}

func loadInlineTrustVectorCsv(
	filename string, ref *openapi.TrustVectorRef,
) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer util.Close(f)
	reader := csv.NewReader(f)
	inputErrorf := func(field int, format string, v ...interface{}) error {
		line, column := reader.FieldPos(0)
		return fmt.Errorf("%s:%d:%d: %s",
			filename, line, column, fmt.Sprintf(format, v...))
	}
	inputWrapf := func(
		err error, field int, format string, v ...interface{},
	) error {
		line, column := reader.FieldPos(0)
		return fmt.Errorf("%s:%d:%d: %s: %w", filename, line, column,
			fmt.Sprintf(format, v...), err)
	}
	inline := openapi.InlineTrustVector{
		Scheme:  openapi.InlineTrustVectorSchemeInline,
		Entries: nil,
		Size:    0,
	}
	ignoreFirst := csvHasHeader
	fields, err := reader.Read()
	for ; err == nil; fields, err = reader.Read() {
		if len(fields) < 1 {
			return inputErrorf(0, "too few (%d) fields", len(fields))
		}
		if len(fields) > 2 {
			return inputErrorf(0, "too many (%d) fields", len(fields))
		}
		if ignoreFirst {
			ignoreFirst = false
			continue
		}
		var (
			from  int
			value float64
		)
		from, err = getPeerIndex(fields[0])
		switch {
		case err != nil:
			return inputWrapf(err, 0, "invalid from=%#v", fields[0])
		case from < 0:
			return inputErrorf(0, "negative from=%#v", from)
		}
		value, err = strconv.ParseFloat(fields[1], 64)
		switch {
		case err != nil:
			return inputWrapf(err, 1, "invalid trust value=%#v", fields[1])
		case value < 0:
			return inputErrorf(1, "negative value=%#v", value)
		}
		inline.Entries = append(inline.Entries,
			openapi.InlineTrustVectorEntry{I: from, V: value})
		if inline.Size <= from {
			inline.Size = from + 1
		}
	}
	if inline.Size == 0 {
		return errors.New("empty trust vector")
	}
	if err != io.EOF {
		return fmt.Errorf("cannot read trust vector CSV %#v: %w", filename, err)
	}
	if err = ref.FromInlineTrustVector(inline); err != nil {
		return fmt.Errorf("cannot wrap inline trust vector: %w", err)
	}
	ref.Scheme = openapi.Inline
	return nil
}

func writeOutput(
	entries []openapi.InlineTrustVectorEntry, filename string,
) error {
	file, err := util.OpenOutputFile(filename)
	if err != nil {
		return fmt.Errorf("cannot open output file: %w", err)
	}
	defer util.Close(file)
	csvWriter := csv.NewWriter(file)
	for _, entry := range entries {
		var peerId string
		peerId, err = getPeerId(entry.I)
		if err != nil {
			return err
		}
		if err = csvWriter.Write([]string{
			peerId,
			strconv.FormatFloat(entry.V, 'f', -1, 64),
		}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	if csvWriter.Error() != nil {
		return fmt.Errorf("cannot flush output file: %w", err)
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
	client, err := openapi.NewClientWithResponses(endpoint)
	if err != nil {
		logger.Err(err).Msg("cannot create an API client")
		return
	}
	ctx := context.Background()
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
		var preTrustRef openapi.TrustVectorRef
		err = trustVectorURIToRef(preTrustURI, &preTrustRef)
		if err != nil {
			logger.Err(err).Msg("cannot parse/load pre-trust reference")
			return
		}
		requestBody.PreTrust = &preTrustRef
	}
	if initialTrustURI != "" {
		var initialTrustRef openapi.TrustVectorRef
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
		}{&requestBody, peerIds}
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
		} else if inlineEigenTrust, err := resp.JSON200.EigenTrust.AsInlineTrustVector(); err != nil {
			logger.Error().Msg("cannot parse response")
		} else {
			if err = writeOutput(
				inlineEigenTrust.Entries, outputFilename,
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

func getPeerIndex(peerId string) (peerIndex int, err error) {
	if rawPeerIds {
		i64, e := strconv.ParseInt(peerId, 0, 0)
		peerIndex, err = int(i64), e
	} else if existing, ok := peerIndices[peerId]; ok {
		peerIndex = existing
	} else {
		peerIndex = len(peerIds)
		peerIds = append(peerIds, peerId)
		peerIndices[peerId] = peerIndex
	}
	return
}

func getPeerId(peerIndex int) (peerId string, err error) {
	if rawPeerIds {
		peerId = strconv.FormatInt(int64(peerIndex), 10)
	} else if peerIndex < len(peerIds) {
		peerId = peerIds[peerIndex]
	} else {
		err = fmt.Errorf("unknown peer index %d", peerIndex)
	}
	return
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
	peerIndices = make(map[string]int)
}
