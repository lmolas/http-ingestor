// Original code with Dispatcher
package main

import (
	"flag"
	"log"
	"net/http"

	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Job holds the attributes needed to perform unit of work.
type Job struct {
	Name  string
	Delay time.Duration
}

// NewWorker creates takes a numeric id and a channel w/ worker pool.
func NewWorker(id int, workerPool chan chan Job) Worker {
	return Worker{
		id:         id,
		jobQueue:   make(chan Job),
		workerPool: workerPool,
		quitChan:   make(chan bool),
	}
}

type Worker struct {
	id         int
	jobQueue   chan Job
	workerPool chan chan Job
	quitChan   chan bool
}

func (w Worker) start() {
	go func() {
		for {
			// Add my jobQueue to the worker pool.
			w.workerPool <- w.jobQueue

			select {
			case job := <-w.jobQueue:
				// Dispatcher has added a job to my jobQueue.
				// log.Printf("worker%d: started %s, blocking for %f seconds\n", w.id, job.Name, job.Delay.Seconds())
				// log.Printf("worker%d: started %s, blocking for %f seconds\n", w.id, job.Name, job.Delay.Seconds())
				time.Sleep(job.Delay)
				// log.Printf("worker%d: completed %s!\n", w.id, job.Name)
				// log.Printf("worker%d: completed %s!\n", w.id, job.Name)
			case <-w.quitChan:
				// We have been asked to stop.
				log.Printf("worker%d stopping\n", w.id)
				log.Printf("worker%d stopping\n", w.id)
				return
			}
		}
	}()
}

func (w Worker) stop() {
	go func() {
		w.quitChan <- true
	}()
}

// NewDispatcher creates, and returns a new Dispatcher object.
func NewDispatcher(jobQueue chan Job, maxWorkers int) *Dispatcher {
	workerPool := make(chan chan Job, maxWorkers)

	return &Dispatcher{
		jobQueue:   jobQueue,
		maxWorkers: maxWorkers,
		workerPool: workerPool,
	}
}

type Dispatcher struct {
	workerPool chan chan Job
	maxWorkers int
	jobQueue   chan Job
	workers    []*Worker
}

func (d *Dispatcher) run() {
	for i := 0; i < d.maxWorkers; i++ {
		worker := NewWorker(i+1, d.workerPool)
		d.workers = append(d.workers, &worker)
		log.Printf("Start worker %d", worker.id)
		worker.start()
	}

	go d.dispatch()
}

func (d *Dispatcher) dispatch() {
	for {
		select {
		case job := <-d.jobQueue:
			go func() {
				// log.Printf("fetching workerJobQueue for: %s\n", job.Name)
				workerJobQueue := <-d.workerPool
				// log.Printf("adding %s to workerJobQueue\n", job.Name)
				workerJobQueue <- job
			}()
		}
	}
}

func (d *Dispatcher) stop() {
	log.Printf("Stopping %d workers\n", len(d.workers))
	for _, w := range d.workers {
		w.stop()
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request, jobQueue chan Job) {
	// Make sure we can only be called with an HTTP POST request.
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// body, err := ioutil.ReadAll(r.Body)
	// if err != nil {
	// 	panic(err)
	// }
	// log.Printf("body = %s\n", string(body))

	// defer r.Body.Close()

	// Parse the delay.
	// delay, err := time.ParseDuration(r.FormValue("delay"))
	// if err != nil {
	// 	log.Printf("Bad delay value %s\n", err.Error())
	// 	http.Error(w, "Bad delay value: "+err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// // Validate delay is in range 1 to 10 seconds.
	// if delay.Seconds() < 1 || delay.Seconds() > 10 {
	// 	log.Printf("The delay must be between 1 and 10 seconds, inclusively.\n")
	// 	http.Error(w, "The delay must be between 1 and 10 seconds, inclusively.", http.StatusBadRequest)
	// 	return
	// }

	// // Set name and validate value.
	// name := r.FormValue("name")
	// if name == "" {
	// 	log.Printf("You must specify a name.\n")
	// 	http.Error(w, "You must specify a name.", http.StatusBadRequest)
	// 	return
	// }

	name := "Job"
	delay := 20 * time.Millisecond

	// Create Job and push the work onto the jobQueue.
	job := Job{Name: name, Delay: delay}
	jobQueue <- job

	// Render success.
	w.WriteHeader(http.StatusCreated)
}

type Service struct {
	Server     *http.Server
	Dispatcher *Dispatcher
	JobQueue   chan Job
}

func main() {
	var (
		maxWorkers   = flag.Int("max_workers", 20, "The number of workers to start")
		maxQueueSize = flag.Int("max_queue_size", 100, "The size of job queue")
		port         = flag.String("port", "8080", "The server port")
	)
	flag.Parse()

	// Create things
	jobQueue := make(chan Job, *maxQueueSize)
	dispatcher := NewDispatcher(jobQueue, *maxWorkers)
	server := &http.Server{Addr: ":" + *port}

	service := &Service{
		Server:     server,
		Dispatcher: dispatcher,
		JobQueue:   jobQueue,
	}

	// start things
	go startHttp(service, *maxWorkers, *maxQueueSize, *port)

	// Capture signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
	}()

	//Wait for the end
	select {
	case <-c:
		log.Printf("Termination signal received ...\n")
		func() {
			stopHttp(service)
		}()

	}
}

func startHttp(service *Service, maxWorkers, maxQueueSize int, port string) error {

	// Start the dispatcher.
	service.Dispatcher.run()

	// Start the HTTP handler.
	http.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		requestHandler(w, r, service.JobQueue)
	})

	return service.Server.ListenAndServe()
	//return http.ListenAndServe(":"+port, nil)
}

func stopHttp(service *Service) {
	service.Server.Close()
	service.Dispatcher.stop()
}
