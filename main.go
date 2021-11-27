package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/toorop/msfs2020-simconnect-go/simconnect"
)

// const version = 1.1.1

type SimVar struct {
	DefineID   simconnect.DWord
	Name, Unit string
}

type SimObjectValue struct {
	simconnect.RecvSimObjectDataByType
	Value float64
}

var (
	requestDataInterval = time.Millisecond * 350
	//receiveDataInterval = time.Millisecond * 10
	simConnect  *simconnect.SimConnect
	cameraState simconnect.DWord
	debounce    time.Time

	//go:embed resources/SimConnect.dll
	dllEmbedded []byte
)

const (
//lftMax = float32(3.402823e+38)
)

const (
	DataDefTrackIrEnable = iota
	DataDefCameraState
)

func main() {
	// embedded the binary for the dll into the main binary.

	if err := checkDLL(); err != nil {
		log.Fatal(err)
	}

	if err := simconnect.Initialize(""); err != nil {
		panic(err)
	}

	// todo wait for simconnect to be ready
	simConnect = simconnect.NewSimConnect()

	// wait and connect
	waitForSimAndConnect()

	// register event DataDefTrackIrEnable
	if err := addToDataDefinition(&SimVar{
		DefineID: DataDefTrackIrEnable,
		Name:     "TRACK IR ENABLE",
		Unit:     "Bool",
	}); err != nil {
		log.Fatalln(err)
	}

	// register event DataDefSimDisabled
	if err := addToDataDefinition(&SimVar{
		DefineID: DataDefCameraState,
		Name:     "CAMERA STATE",
		Unit:     "Enum",
	}); err != nil {
		log.Fatalln(err)
	}

	done := make(chan bool, 1)
	defer close(done)
	go HandleTerminationSignal(done)
	go HandleEvents(done)

	<-done

	if err := simConnect.Close(); err != nil {
		panic(err)
	}
}

func checkDLL() error {
	exPath, err := getExePath()
	if err != nil {
		return err
	}
	dllPath := filepath.Join(exPath, "SimConnect.dll")
	if _, err = os.Stat(dllPath); err != nil {
		// dll not found
		log.Println("SimConnect.dll not found. I will install it.")
		fd, err := os.OpenFile(dllPath, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		defer func(fd *os.File) {
			_ = fd.Close()
		}(fd)
		_, err = fd.Write(dllEmbedded)
		return err
	}
	return nil
}

func waitForSimAndConnect() {
	log.Print("Waiting for simulator to be ready...")
	for {
		err := simConnect.Open("AutoTrackIr")
		if err != nil {
			fmt.Print(".")
			time.Sleep(time.Second * 1)
		} else {
			fmt.Println()
			break
		}
	}
	//fmt.Println()
}

func HandleTerminationSignal(done chan bool) {
	sigterm := make(chan os.Signal, 1)
	defer close(sigterm)
	signal.Notify(sigterm, os.Interrupt, syscall.SIGTERM)
	<-sigterm
	done <- true
}

func HandleEvents(done chan bool) {
	reqDataTicker := time.NewTicker(requestDataInterval)
	defer reqDataTicker.Stop()

	//recvDataTicker := time.NewTicker(receiveDataInterval)
	//defer recvDataTicker.Stop()

	var simObjectType = simconnect.SimObjectTypeUser
	var radius = simconnect.DWordZero

	for {
		<-reqDataTicker.C
		_ = simConnect.RequestDataOnSimObjectType(simconnect.NewRequestID(), DataDefTrackIrEnable, radius, simObjectType)
		dispatchProc(done)
		_ = simConnect.RequestDataOnSimObjectType(simconnect.NewRequestID(), DataDefCameraState, radius, simObjectType)
		dispatchProc(done)
	}
}

// addToDataDefinition adds a SimVar to the data definition.
func addToDataDefinition(simVar *SimVar) error {
	if err := simConnect.AddToDataDefinition(simVar.DefineID, simVar.Name, simVar.Unit, simconnect.DataTypeFloat64); err != nil {
		return fmt.Errorf("addToDataDefinition %d failed: %s", simVar.DefineID, err)
	}
	return nil
}

func dispatchProc(done chan bool) {

	ppData, r1, err := simConnect.GetNextDispatch()
	if r1 < 0 {
		if uint32(r1) != simconnect.EFail {
			fmt.Printf("GetNextDispatch error: %d %s\n", r1, err)
		}
		/*if ppData == nil {
			// todo
		}*/
		return
	}

	recv := *(*simconnect.Recv)(ppData)
	switch recv.ID {
	case simconnect.RecvIDOpen:
		log.Println("Connected to Flight Simulator.")
		dispatchProc(done)

	case simconnect.RecvIDQuit:
		log.Println("Disconnected from Flight Simulator.")
		done <- true

	case simconnect.RecvIDException:
		recvException := *(*simconnect.RecvException)(ppData)
		log.Println("Error:", recvException.Exception)
		dispatchProc(done)

	case simconnect.RecvIDSimObjectDataByType:
		var logIt bool
		data := *(*SimObjectValue)(ppData)
		//fmt.Printf("[%d] %s %s %f\n", data.RequestID, simVar.Name, simVar.Unit, data.Value)
		if data.DefineID == DataDefTrackIrEnable {
			if data.Value == 0 && cameraState > 1 && cameraState < 6 {
				objectID := data.RecvSimObjectDataByType.ObjectID
				if time.Since(debounce) < 10*time.Second {
					logIt = false
				} else {
					debounce = time.Now()
					logIt = true
				}
				if logIt {
					log.Println("Track IR disabled by sim")
				}
				data := [1]float64{1}
				size := simconnect.DWord(unsafe.Sizeof(data))
				err := simConnect.SetDataOnSimObject(DataDefTrackIrEnable, objectID, simconnect.DataSetFlagDefault, 0, size, unsafe.Pointer(&data))
				if logIt {
					log.Println("TracIR enabled by me")
				}
				if err != nil {
					log.Println("Error:", err)
				}
			}
		} else if data.DefineID == DataDefCameraState {
			//log.Printf("[%d] Camera STATE  %f\n", data.RequestID, data.Value)
			cameraState = simconnect.DWord(data.Value)
		}
	default:
		fmt.Printf("recieved: %v\n", ppData)
	}
}

func getExePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}
