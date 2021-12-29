package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	bwav "github.com/faiface/beep/wav"
	prometheus "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/youpy/go-wav"
)

var (
	prometheusURL = flag.String("prom_url", "", "Promtheus URL")
	offset        = flag.Duration("offset", 24*time.Hour, "Query metrics from time.Now() - offset if startTime is not set")
	startTime     = flag.String("start_time", "", "Query metrics from the start time, if not set, use offset instead")
	endTime       = flag.String("end_time", "", "Query metrics to the end time, if not set, use now")
	trackName     = flag.String("track", "", "The query you want to send to the Prometheus")
	chunkSize     = flag.Duration("chunk", time.Minute, "Query time range for each query")
)

const (
	TIME_FORMAT = "2006-01-02 15:04:05"
)

func panicError(err error) {
	if err == nil {
		return
	}

	panic(err)
}

// refer to https://github.com/MacroPower/prometheus_video_renderer
func getAudioChunk(
	q v1.API,
	startTime time.Time,
	trackName string,
	scrapeInterval int,
	chunkSize time.Duration,
	n int,
) []wav.Sample {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	value, _, err := q.QueryRange(ctx, trackName, v1.Range{
		Start: startTime.Add(chunkSize * time.Duration(n+0)),
		End:   startTime.Add(chunkSize * time.Duration(n+1)),
		Step:  time.Duration(scrapeInterval) * time.Second * 1,
	})
	panicError(err)

	queryType := value.Type()

	if queryType == model.ValMatrix {
		samples := make([]wav.Sample, 0, 1024)

		matrixVal := value.(model.Matrix)
		for _, series := range matrixVal {
			for _, elem := range series.Values {
				samples = append(samples, wav.Sample{Values: [2]int{int(elem.Value), int(elem.Value)}})
			}
		}

		if len(samples) == 0 {
			return nil
		}

		fmt.Printf("Read %d samples, chunk %d\n", len(samples), n)
		return samples
	}

	return nil
}

func calTimeRange() (startT time.Time, endT time.Time) {
	now := time.Now()
	startT = parseTime(*startTime, now.Add(-*offset))
	endT = parseTime(*endTime, now)

	if endT.After(now) {
		endT = now
	}

	if endT.Before(startT) {
		startT = endT.Add(-*offset)
	}
	return
}

func playMetrics() {
	// 44100 is a common sampling frequency, widely used due to the compact disc format
	// Refer to https://en.wikipedia.org/wiki/44,100_Hz
	var sampleRate int = 44100
	sr := beep.SampleRate(sampleRate)
	speaker.Init(sr, sr.N(time.Second/10))

	var queue Queue
	speaker.Play(&queue)

	var numChannels uint16 = 1
	var bitsPerSample uint16 = 8

	scrapeInterval := 1
	startT, endT := calTimeRange()
	step := int(endT.Sub(startT) / *chunkSize)

	client, err := prometheus.NewClient(prometheus.Config{Address: *prometheusURL})
	panicError(err)

	q := v1.NewAPI(client)

	for i := 0; i <= step; i++ {
		data := getAudioChunk(q, startT, *trackName, scrapeInterval, *chunkSize, i)

		if len(data) == 0 {
			continue
		}

		b := new(bytes.Buffer)
		writer := wav.NewWriter(b, uint32(len(data)), numChannels, uint32(sampleRate), bitsPerSample)
		writer.WriteSamples(data)

		streamer, format, err := bwav.Decode(b)
		panicError(err)

		// 4 is a reasonable value for resampling
		// Refer to https://pkg.go.dev/github.com/faiface/beep#Resample
		resampled := beep.Resample(4, format.SampleRate, sr, streamer)

		speaker.Lock()
		queue.Add(resampled)
		speaker.Unlock()
	}

	done := make(chan bool)
	queue.Add(beep.Callback(func() {
		done <- true
	}))

	<-done
}

type Queue struct {
	streamers []beep.Streamer
	count     int
}

func (q *Queue) Add(streamers ...beep.Streamer) {
	q.streamers = append(q.streamers, streamers...)
}

func (q *Queue) Stream(samples [][2]float64) (n int, ok bool) {
	// We use the filled variable to track how many samples we've
	// successfully filled already. We loop until all samples are filled.
	filled := 0
	for filled < len(samples) {
		// There are no streamers in the queue, so we stream silence.
		if len(q.streamers) == 0 {
			for i := range samples[filled:] {
				samples[i][0] = 0
				samples[i][1] = 0
			}
			break
		}

		// We stream from the first streamer in the queue.
		n, ok := q.streamers[0].Stream(samples[filled:])
		// If it's drained, we pop it from the queue, thus continuing with
		// the next streamer.
		if !ok {
			q.streamers = q.streamers[1:]

			// fmt.Printf("Finished playing chunk %d\n", q.count)
			q.count++
		}
		// We update the number of filled samples.
		filled += n
	}
	return len(samples), true
}

func (q *Queue) Err() error {
	return nil
}

func parseTime(s string, v time.Time) time.Time {
	if s == "" {
		return v
	}

	if t, err := time.Parse(TIME_FORMAT, s); err == nil {
		return t
	}

	return v
}

func main() {
	flag.Parse()

	if *trackName == "" {
		println("please set a query to query metrics from Prometheus")
		flag.PrintDefaults()
		return
	}

	if *prometheusURL == "" {
		println("please assign a Prometheus URL")
		flag.PrintDefaults()
		return
	}

	playMetrics()
}
