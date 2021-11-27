# AutoTrackIR
AutoTrackIR will automatically re-enable your Track IR if flight simulator disabled it. (bug introduce in SU7)

## How it works ?
It just watches for "TRACK IR ENABLE" simconnect variable.
If the simulator switch this variable to FALSE then AutoTrackIR switch it back to TRUE.


## How to use it ?
- launch the TrackIr application.
- launch your sim
- when you are on you plane and your TrackIR is active juste launch autoTrackIr.exe
- enjoy !

## How to update it ?
- replace the autoTrackIr.exe file with the new one
- restart the application

## How to uninstall it ?
- just delete the autoTrackIr.exe file

## how to build it from source ?
- install Golang
- download the latest version of the source code from GitHub
- add the simconnect.dll file to the project in the resources folder 
- go to the project folder
- run the following command :
 ```
go build -o autoTrackIr.exe
````
