package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

type message struct {
	PhoneNumber string
	MessageText string
	ModemID     string
}

type Modems struct {
	ModemIds []string
	Mutex    sync.Mutex
}

var shutdownWaitGroup = sync.WaitGroup{}
var modems Modems

func main() {
	modemQueue := make(chan message, 100)
	shutdown := make(chan struct{})
	done := make(chan struct{})

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down gracefully...")
		close(shutdown)
	}()

	// Enable modems before attempting to send SMS
	go enableModems(shutdown)

	// wait for modems to be enabled at the first start
	time.Sleep(10 * time.Second)

	// Start workers for each modem
	go startWorkers(modemQueue, shutdown)

	// Main processing loop - enqueue messages
	go func() {
		defer close(done)
		for {
			select {
			case <-shutdown:
				// Close the queue to signal workers to finish
				close(modemQueue)
				return
			default:
				if len(modems.ModemIds) == 0 {
					time.Sleep(3 * time.Second)
					continue
				}

				// Enqueue messages for each modem
				for _, modemID := range modems.ModemIds {
					msg, err := getMessage()
					msg.ModemID = modemID

					if err != nil {
						fmt.Printf("Error getting message: %v\n", err)
						continue
					}

					select {
					case modemQueue <- msg:
						fmt.Printf("Enqueued message for modem %s (queue size: %d)\n", modemID, len(modemQueue))
					case <-shutdown:
						close(modemQueue)
						return
					default:
						fmt.Println("Queue is full, waiting...")
						time.Sleep(1 * time.Second)
					}
				}

				time.Sleep(3 * time.Second)
			}
		}
	}()

	// Wait for shutdown
	<-done
	fmt.Println("Main loop closed, waiting for workers to finish...")
	shutdownWaitGroup.Wait()
	fmt.Println("All workers are done")
	fmt.Println("Shutdown complete")
}

// getMessage retrieves an SMS message to be sent, typically from a database or API.
func getMessage() (message, error) {
	return message{
		PhoneNumber: "+99361041499",
		MessageText: "Hello, world!",
	}, nil
}

// startWorkers manages worker goroutines for processing SMS queue
func startWorkers(modemQueue chan message, shutdown chan struct{}) {
	workerCount := 3 // Number of concurrent workers
	fmt.Printf("Starting %d SMS workers...\n", workerCount)

	for i := range workerCount {
		shutdownWaitGroup.Add(1)
		go smsWorker(i+1, modemQueue, shutdown)
	}
}

// smsWorker processes messages from the queue
func smsWorker(workerID int, modemQueue chan message, shutdown chan struct{}) {
	defer shutdownWaitGroup.Done()
	fmt.Printf("Worker %d started\n", workerID)

	for {
		select {
		case <-shutdown:
			// Drain remaining messages before shutting down
			for msg := range modemQueue {
				fmt.Printf("Worker %d draining message for modem %s\n", workerID, msg.ModemID)
				err := sendSMS(msg.PhoneNumber, msg.MessageText, msg.ModemID)

				if err != nil {
					fmt.Printf("Worker %d error sending SMS via modem %s: %v\n", workerID, msg.ModemID, err)
				} else {
					fmt.Printf("Worker %d successfully sent SMS via modem %s to %s\n", workerID, msg.ModemID, msg.PhoneNumber)
				}
			}
			fmt.Printf("Worker %d shutting down\n", workerID)
			return
		case msg, ok := <-modemQueue:
			if !ok {
				// Channel closed
				fmt.Printf("Worker %d detected closed channel, shutting down\n", workerID)
				return
			}

			fmt.Printf("Worker %d processing message for modem %s\n", workerID, msg.ModemID)
			err := sendSMS(msg.PhoneNumber, msg.MessageText, msg.ModemID)
			if err != nil {
				fmt.Printf("Worker %d error sending SMS via modem %s: %v\n", workerID, msg.ModemID, err)
			} else {
				fmt.Printf("Worker %d successfully sent SMS via modem %s to %s\n", workerID, msg.ModemID, msg.PhoneNumber)
			}
		}
	}
}

// create sms and send it
func sendSMS(phoneNumber, messageText, modemID string) error {
	createSMSCommand := fmt.Sprintf("mmcli -m %s --messaging-create-sms=\"text='%s',number='%s'\"", modemID, messageText, phoneNumber)
	cmd := exec.Command("bash", "-c", createSMSCommand)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	if err != nil {
		return err
	}

	output := out.String()
	var smsID string

	for line := range strings.SplitSeq(output, "\n") {

		if strings.Contains(line, "SMS") {
			parts := strings.Fields(line)

			if len(parts) > 0 {
				smsID = strings.TrimSpace(parts[len(parts)-1])
				break
			}
		}
	}

	if smsID == "" {
		return errors.New("sms id not found")
	}

	sendSMSCommand := fmt.Sprintf("mmcli -s %s --send", smsID)
	cmd = exec.Command("bash", "-c", sendSMSCommand)
	out.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()

	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)
	return nil
}

// The loop continuously enables modems, handling cases where modems are added, removed, or their ports are changed.
func enableModems(shutdown chan struct{}) error {
	for {
		select {
		case <-shutdown:
			return errors.New("shutdown")
		default:
			// 1. List all modems
			cmd := exec.Command("mmcli", "-L")
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			err := cmd.Run()

			if err != nil {
				return err
			}

			output := out.String()

			if output == "No modems were found\n" {
				return errors.New("no modems were found")
			}

			for line := range strings.SplitSeq(output, "\n") {

				if strings.Contains(line, "Modem") {
					parts := strings.Fields(line)

					if len(parts) > 0 {
						modemID := strings.TrimSpace(parts[0])
						modems.Mutex.Lock()
						if !slices.Contains(modems.ModemIds, modemID) {
							modems.ModemIds = append(modems.ModemIds, modemID)
						}
						modems.Mutex.Unlock()
					}
				}
			}

			if len(modems.ModemIds) == 0 {
				return errors.New("no modems were found")
			}

			// enable modems
			for _, modemID := range modems.ModemIds {
				enableCommand := fmt.Sprintf("mmcli -m %s --enable", modemID)
				cmd = exec.Command("bash", "-c", enableCommand)
				out.Reset()
				cmd.Stdout = &out
				cmd.Stderr = &out
				err = cmd.Run()

				if err != nil {
					return err
				}
			}

			time.Sleep(15 * time.Second)
		}
	}
}
