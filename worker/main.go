package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/airbusgeo/cogger"
	"github.com/airbusgeo/godal"
	"github.com/airbusgeo/osio"
	"github.com/airbusgeo/osio/gcs"
	"github.com/sourcegraph/conc/pool"
)

func median(inputs [][]uint8) []uint8 {
	result := make([]uint8, len(inputs[0]))
	bandLen := len(inputs[0]) / 3
	sample := make([][3]uint8, len(inputs))
	for pix := 0; pix < bandLen; pix++ {
		count := 0
		for s := range inputs {
			if inputs[s][pix] == 0 || inputs[s][pix+bandLen] == 0 || inputs[s][pix+2*bandLen] == 0 ||
				inputs[s][pix] == 255 || inputs[s][pix+bandLen] == 255 || inputs[s][pix+2*bandLen] == 255 { //nodata or saturated
				continue
			}
			sample[count][0] = inputs[s][pix]
			sample[count][1] = inputs[s][pix+bandLen]
			sample[count][2] = inputs[s][pix+2*bandLen]
			count++
		}
		if count == 0 {
			continue
		}
		sort.Slice(sample[0:count], func(i, j int) bool {
			return int(sample[i][0])+int(sample[i][1])+int(sample[i][2]) < int(sample[j][0])+int(sample[j][1])+int(sample[j][2])
		})

		med := count / 2
		result[pix] = sample[med][0]
		result[pix+bandLen] = sample[med][1]
		result[pix+2*bandLen] = sample[med][2]
	}
	return result
}

func getbuffer(dataset string, window [4]int) ([]uint8, error) {
	ds, err := godal.Open(dataset)
	if err != nil {
		return nil, fmt.Errorf("open dataset: %w", err)
	}
	defer ds.Close()
	s := ds.Structure()
	if s.NBands != 3 {
		return nil, fmt.Errorf("expecting 3 bands, got %d", s.NBands)
	}
	if window[0] < 0 || window[1] < 0 || window[0]+window[2] > s.SizeX || window[1]+window[3] > s.SizeY {
		return nil, fmt.Errorf("window out of bounds")
	}
	buf := make([]uint8, window[2]*window[3]*3) //3 bands

	err = ds.Read(window[0], window[1], buf, window[2], window[3], godal.BandInterleaved())
	return buf, err
}

func getbuffers(datasets []string, window [4]int) ([][]uint8, error) {
	buffers := make([][]uint8, len(datasets))
	pool := pool.New().WithErrors().WithMaxGoroutines(10)
	for i := range buffers {
		i := i
		pool.Go(func() error {
			buf, err := getbuffer(datasets[i], window)
			if err != nil {
				return err
			}
			buffers[i] = buf
			return nil
		})
	}
	if err := pool.Wait(); err != nil {
		return nil, err
	}
	return buffers, nil
}

func processRequest(ctx context.Context, r MRequest) error {
	ds0, err := godal.Open(r.Datasets[0])
	if err != nil {
		return fmt.Errorf("open dataset: %w", err)
	}
	gt, _ := ds0.GeoTransform()
	srs := ds0.Projection()
	ds0.Close()

	bufs, err := getbuffers(r.Datasets, r.Window)
	if err != nil {
		return fmt.Errorf("getbuffers: %w", err)
	}
	result := median(bufs)
	tmpfile, err := os.CreateTemp("", "m*.tif")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())
	ds, err := godal.Create(godal.GTiff, tmpfile.Name(), 3, godal.Byte, r.Window[2], r.Window[3],
		godal.CreationOption("COMPRESS=LZW", "TILED=YES"),
	)
	if err != nil {
		return fmt.Errorf("create dataset: %w", err)
	}
	gt[0] += float64(r.Window[0]) * gt[1]
	gt[3] += float64(r.Window[1]) * gt[5]
	ds.SetGeoTransform(gt)
	ds.SetProjection(srs)
	ds.Write(0, 0, result, r.Window[2], r.Window[3], godal.BandInterleaved())
	ds.BuildOverviews(godal.Resampling(godal.Average))
	if err := ds.Close(); err != nil {
		return fmt.Errorf("close dataset: %w", err)
	}

	dst := strings.TrimPrefix(r.Destination, "gs://")
	sep := strings.Index(dst, "/")
	bucket := dst[:sep]
	obj := dst[sep+1:]

	tifReader, err := os.Open(tmpfile.Name())
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer tifReader.Close()

	w := gsClient.Bucket(bucket).Object(obj).NewWriter(ctx)
	if err != nil {
		return fmt.Errorf("create writer for gs://%s/%s: %w", bucket, obj, err)
	}
	if err := cogger.Rewrite(w, tifReader); err != nil {
		return fmt.Errorf("cogger.Rewrite: %w", err)
	}
	if _, err := io.Copy(w, tifReader); err != nil {
		return fmt.Errorf("copy to gs://%s/%s: %w", bucket, obj, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close gs://%s/%s: %w", bucket, obj, err)
	}
	log.Printf("wrote to gs://%s/%s", bucket, obj)
	return nil
}

type MRequest struct {
	Datasets    []string `json:"datasets"`
	Window      [4]int   `json:"window"`
	Destination string   `json:"destination"`
}

type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data,omitempty"`
		ID   string `json:"id"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func MedianHandler(w http.ResponseWriter, r *http.Request) {
	var m PubSubMessage
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ioutil.ReadAll: %v", err)
		return
	}
	r.Body.Close()
	// byte slice unmarshalling handles base64 decoding.
	if err := json.Unmarshal(body, &m); err != nil {
		log.Printf("json.Unmarshal: %v", err)
		return
	}
	var mrequest MRequest
	if err := json.Unmarshal(m.Message.Data, &mrequest); err != nil {
		log.Printf("json.Unmarshal: %v", err)
		return //200 to ack the message
	}
	log.Printf("received request: %v", mrequest.Window)
	if err := processRequest(r.Context(), mrequest); err != nil {
		log.Printf("processRequest failed: %v", err)
		return
	}
}

var gsClient *storage.Client

func main() {
	ctx := context.Background()
	godal.RegisterInternalDrivers()
	gcsR, err := gcs.Handle(ctx)
	if err != nil {
		log.Fatal(err)
	}
	osioA, _ := osio.NewAdapter(gcsR,
		osio.BlockSize("64k"),
		osio.NumCachedBlocks(1000),
	)
	godal.RegisterVSIHandler("gs://", osioA)

	gsClient, err = storage.NewClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/median", MedianHandler)
	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}
	// Start HTTP server.
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
