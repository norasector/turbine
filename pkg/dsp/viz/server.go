package viz

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
)

type ImageContainer struct {
	name string
	data []byte
}

type Producer interface {
	Name() string
	GetImage() *ImageContainer
	AddPlotOption(opt PlotOptions)
}

type Server struct {
	recvImage       chan *ImageContainer
	images          map[string]map[string]*ImageContainer
	mu              sync.RWMutex
	port            int
	srv             *http.Server
	producerBuckets map[string]map[string]Producer
	updateInterval  time.Duration
	enabled         bool
	lastViewed      map[string]time.Time
}

func NewServer(port int, updateInterval time.Duration) *Server {
	return &Server{
		images:          make(map[string]map[string]*ImageContainer),
		recvImage:       make(chan *ImageContainer),
		producerBuckets: make(map[string]map[string]Producer),
		port:            port,
		lastViewed:      make(map[string]time.Time),
		srv:             &http.Server{Addr: fmt.Sprintf(":%d", port)},
		updateInterval:  updateInterval,
		enabled:         true,
	}
}

func (s *Server) Enable(enable bool) {
	s.mu.Lock()
	s.enabled = enable
	s.mu.Unlock()
}

func (s *Server) SetUpdateInterval(interval time.Duration) {
	s.mu.Lock()
	s.updateInterval = interval
	s.mu.Unlock()
}

func (s *Server) Register(key string, p Producer) {
	s.mu.Lock()
	bucket, ok := s.producerBuckets[key]
	if !ok {
		bucket = make(map[string]Producer)
		s.producerBuckets[key] = bucket
	}
	bucket[p.Name()] = p
	s.mu.Unlock()

}

func (s *Server) Receive(img *ImageContainer) {
	s.recvImage <- img
}

func (s *Server) Stop(ctx context.Context) {
	s.srv.Shutdown(ctx)
}

func (s *Server) Run(ctx context.Context) error {

	go func() {
		for {
			select {

			case <-ctx.Done():
				return
			case <-time.After(s.updateInterval):
				if !s.enabled {
					continue
				}

				s.mu.Lock()
				buckets := s.producerBuckets
				s.mu.Unlock()
				var wg sync.WaitGroup

				for bucketName, bucket := range buckets {

					s.mu.Lock()
					lastViewed := s.lastViewed[bucketName]
					s.mu.Unlock()
					if time.Since(lastViewed) < time.Second {
						for _, producer := range bucket {
							wg.Add(1)
							go func(bucket string, p Producer) {
								defer wg.Done()

								s.mu.Lock()

								img := p.GetImage()
								if img != nil {
									mb, ok := s.images[bucket]
									if !ok {
										mb = make(map[string]*ImageContainer)
										s.images[bucket] = mb
									}
									mb[img.name] = img
								}

								s.mu.Unlock()

							}(bucketName, producer)
						}
					}
				}
				wg.Wait()
			}
		}
	}()

	handler := httprouter.New()
	handler.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var key string
		s.mu.RLock()
		for name := range s.producerBuckets {
			key = name
			break
		}
		defer s.mu.RUnlock()

		w.Header().Set("Location", url.PathEscape(fmt.Sprintf("/view/%s", key)))
		w.WriteHeader(http.StatusFound)

	})

	handler.GET("/view/:bucket", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		bucket := params.ByName("bucket")

		itemsForBucket, ok := s.producerBuckets[bucket]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		s.mu.Lock()
		s.lastViewed[bucket] = time.Now()
		s.mu.Unlock()

		time.Sleep(s.updateInterval)

		s.mu.RLock()
		defer s.mu.RUnlock()

		w.Header().Add("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Turbine Viz</title></head>`))

		w.Write([]byte(fmt.Sprintf(`
		<script type="text/javascript">
			var toggleRefresh = true;
			function toggleOn() {
				toggleRefresh = !toggleRefresh;
			}

			function changeBucket() {
				var val = document.getElementById('bucketSelector').value;
				window.location.href = '/view/' + val;
			}
			window.onload = function() {
				for (var i = 0; i < %d; i++) {
					var img = document.getElementById('graph-' + i);
					setInterval(function(image) {
						if (toggleRefresh) {
							image.src = image.src.split("?")[0] + "?" + new Date().getTime();
						}
					}, %d, img);
				}

			}
		</script>`, len(s.producerBuckets), s.updateInterval.Milliseconds())))
		w.Write([]byte(`<body style='background-color: black'>`))

		keys := make([]string, 0, len(s.producerBuckets))
		for key := range s.producerBuckets {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		// bucketKeys := s.producerBuckets

		w.Write([]byte(`<select id="bucketSelector" onchange="changeBucket()">`))
		for _, bucketName := range keys {
			selected := ""
			if bucketName == bucket {
				selected = " selected"
			}
			w.Write([]byte(fmt.Sprintf(`<option value="%s"%s>%s</option>`, bucketName, selected, bucketName)))
		}
		w.Write([]byte(`</select>`))
		w.Write([]byte(`<button onclick="toggleOn()">Refresh?</button>`))

		keys = make([]string, 0, len(itemsForBucket))
		for key := range itemsForBucket {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		w.Write([]byte(`<div style="display: flex; flex-direction: row; flex-wrap: wrap">`))
		for idx, key := range keys {
			// imgContents := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(img.data))

			w.Write([]byte(fmt.Sprintf(`<div><img id="graph-%d" 
			src="/img/%s/%s?%d" />`, idx, bucket, key, time.Now().UnixMicro())))

			w.Write([]byte("</div>"))
		}

		w.Write([]byte(`</div>`))

		w.Write([]byte(`</body></html>`))
	})

	handler.GET("/img/:bucket/:img", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		bucketName := params.ByName("bucket")

		s.mu.Lock()
		s.lastViewed[bucketName] = time.Now()
		s.mu.Unlock()

		imgName := params.ByName("img")
		var bucket map[string]*ImageContainer
		var img *ImageContainer
		var ok bool
		func() {
			s.mu.RLock()
			defer s.mu.RUnlock()
			bucket, ok = s.images[bucketName]
			if !ok {
				return
			}
			img, ok = bucket[imgName]
			if !ok {
				return
			}
		}()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "image/png")
		w.Write(img.data)
	})

	s.srv.Handler = handler

	err := s.srv.ListenAndServe()
	switch {
	case err == http.ErrServerClosed:
		return nil
	case err != nil:
		return err
	default:
		return err
	}
}
