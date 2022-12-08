package cmd

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k3l.io/go-eigentrust/pkg/basic"
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
	localTrustURI, preTrustURI string
	alpha, epsilon             float64
)

func localTrustURIToRef(uri string, ref *basic.LocalTrustRef) error {
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
		return loadInlineLocalTrust(path, ref)
	default:
		return errors.Errorf("invalid local trust URI scheme %#v",
			parsed.Scheme)
	}
}

func loadInlineLocalTrust(filename string, ref *basic.LocalTrustRef) error {
	logger.Trace().Str("filename", filename).Msg("loading inline local trust")
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return loadInlineLocalTrustCsv(filename, ref)
	default:
		return errors.Errorf("invalid local trust file type %#v", ext)
	}
}

func loadInlineLocalTrustCsv(filename string, ref *basic.LocalTrustRef) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	reader := csv.NewReader(f)
	inputErrorf := func(field int, format string, v ...interface{}) error {
		line, column := reader.FieldPos(0)
		return errors.Errorf("%s:%d:%d: %s", filename, line, column,
			fmt.Sprintf(format, v...))
	}
	inputWrapf := func(
		err error, field int, format string, v ...interface{},
	) error {
		line, column := reader.FieldPos(0)
		return errors.Wrapf(err, "%s:%d:%d: %s", filename, line, column,
			fmt.Sprintf(format, v...))
	}
	inline := basic.InlineLocalTrust{
		Entries: nil,
		Scheme:  "inline",
		Size:    0,
	}
	fields, err := reader.Read()
	for ; err == nil; fields, err = reader.Read() {
		if len(fields) < 2 {
			return inputErrorf(0, "too few (%d) fields", len(fields))
		}
		if len(fields) > 3 {
			return inputErrorf(0, "too many (%d) fields", len(fields))
		}
		var (
			from, to int64
			value    float64
		)
		from, err = strconv.ParseInt(fields[0], 0, 0)
		switch {
		case err != nil:
			return inputWrapf(err, 0, "invalid from=%#v", fields[0])
		case from < 0:
			return inputErrorf(0, "negative from=%#v", from)
		}
		to, err = strconv.ParseInt(fields[1], 0, 0)
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
		case value < 0:
			return inputErrorf(2, "negative value=%#v", value)
		}
		i, j := int(from), int(to)
		inline.Entries = append(inline.Entries,
			basic.InlineLocalTrustEntry{I: i, J: j, V: value})
		if inline.Size <= i {
			inline.Size = i + 1
		}
		if inline.Size <= j {
			inline.Size = j + 1
		}
	}
	if inline.Size == 0 {
		return errors.New("empty local trust")
	}
	if err != io.EOF {
		return errors.Wrapf(err, "cannot read local trust CSV %#v", filename)
	}
	if err = ref.FromInlineLocalTrust(inline); err != nil {
		return errors.Wrap(err, "cannot wrap inline local trust")
	}
	return nil
}

func runBasicCompute( /*cmd*/ *cobra.Command /*args*/, []string) {
	basicSetupEndpoint()
	var err error
	client, err := basic.NewClientWithResponses(endpoint)
	if err != nil {
		logger.Err(err).Msg("cannot create an API client")
		return
	}
	ctx := context.Background()
	epsilonP := &epsilon
	if epsilon == 0 {
		epsilonP = nil
	}
	requestBody := basic.ComputeJSONRequestBody{
		Alpha:    &alpha,
		Epsilon:  epsilonP,
		PreTrust: nil,
	}
	err = localTrustURIToRef(localTrustURI, &requestBody.LocalTrust)
	if err != nil {
		logger.Err(err).Msg("cannot parse/load local trust reference")
		return
	}
	resp, err := client.ComputeWithResponse(ctx, requestBody)
	if err != nil {
		logger.Err(err).Msg("request failed")
		return
	}
	switch resp.StatusCode() {
	case 200:
		if resp.JSON200 == nil {
			logger.Error().Msg("cannot recover HTTP 200 response")
		} else if inlineEigenTrust, err := resp.JSON200.AsInlineTrustVector(); err != nil {
			logger.Error().Msg("cannot parse response")
		} else {
			for _, entry := range inlineEigenTrust.Entries {
				fmt.Printf("%#v,%#v\n", entry.I, entry.V)
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
	basicComputeCmd.Flags().Float64VarP(&alpha, "alpha", "a", 0.5,
		`Alpha value, between 0.0 and 1.0 inclusive.
Higher value biases the computation toward pre-trust.`)
	basicComputeCmd.Flags().Float64VarP(&epsilon, "epsilon", "e", 0.0,
		`Epsilon (error max).  0 (default) uses server default.`)
}
