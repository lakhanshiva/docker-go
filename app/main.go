package main

import (
	"fmt"
	// Uncomment this block to pass the first stage!
	  "os"
	  "os/exec"
	  "net/http"
	  "syscall"
	  "encoding/json"
	  "io/ioutil"
	  "strings"
	  "bytes"
)

type nullReader struct{}
type nullWriter struct{}

func (nullReader) Read(p []byte) (n int, err error)  { return len(p), nil }
func (nullWriter) Write(p []byte) (n int, err error) { return len(p), nil }

type authToken struct {
	Token string
}

type layer struct {
	BlobSum string
}

func pullLayers(baseDirectory string, fsLayers []layer, imageToPull string, authToken string, dockerExplorerPath string) {
	createDir(baseDirectory, dockerExplorerPath)
	client := &http.Client{}
	for _, layer := range fsLayers {
		url := fmt.Sprintf("https://registry.hub.docker.com/v2/%s/blobs/%s", imageToPull, layer.BlobSum)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
		response, _ := client.Do(req)
		defer response.Body.Close()
		buf := bytes.NewBuffer(make([]byte, 0, response.ContentLength))
		_, _ = buf.ReadFrom(response.Body)
		body := buf.Bytes()
		outputFileName := fmt.Sprintf("%s/%s", baseDirectory, layer.BlobSum)
		err := ioutil.WriteFile(outputFileName, body, 0777)
		if err != nil {
			fmt.Printf("Error while copying docker exp: %v\n", err)
			panic(err)
		}
		tarExtract(outputFileName, baseDirectory)
		removeDanglingFile(outputFileName)
	}
}

func removeDanglingFile(fileName string) {
	args := []string{fileName}
	cmd := exec.Command("rm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	error := cmd.Run()
	if error != nil {
		fmt.Printf("Error while extracting directories: %v\n", error)
		if _, ok := error.(*exec.ExitError); ok {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		fmt.Printf("Err: %v", error)
		os.Exit(1)
	}
}

func tarExtract(outputFileName string, baseDirectory string) {
	args := []string{"-xf", outputFileName, "-C", baseDirectory}
	cmd := exec.Command("tar", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	error := cmd.Run()
	if error != nil {
		fmt.Printf("Error while extracting directories: %v\n", error)
		if _, ok := error.(*exec.ExitError); ok {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		fmt.Printf("Err: %v", error)
		os.Exit(1)
	}
}

func getAuthToken(imageToPull string) string {
	response, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull,push", imageToPull))
	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
	var data authToken
	json.Unmarshal(contents, &data)
	return data.Token
}

func pullManifest(name string, authToken string) []layer {
	getUrl := "https://registry.hub.docker.com/v2/"
	getUrl += name
	getUrl += "/manifests/latest"

	// Create a new request using http
    req, err := http.NewRequest("GET", getUrl, nil)

	// add authorization header to the req
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	// Send req using http Client
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("%s", err)
		os.Exit(1)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        fmt.Printf("%s", err)
		os.Exit(1)
    }

	var objmap map[string]*json.RawMessage
	json.Unmarshal(body, &objmap)

	keys := make([]layer, 0)
	json.Unmarshal(*objmap["fsLayers"], &keys)
	return keys
}

func createDir(baseDirectory string, dockerExplorerPath string) {
	args := []string{"-p", baseDirectory + "/usr/local/bin"}
	cmd := exec.Command("mkdir", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	error := cmd.Run()
	if error != nil {
		fmt.Printf("Error while creating sub-directories: %v\n", error)
		if _, ok := error.(*exec.ExitError); ok {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		fmt.Printf("Err: %v", error)
		os.Exit(1)
	}

	/*pathCreated := baseDirectory + dockerExplorerPath
	args = []string{"-Lr", dockerExplorerPath, pathCreated}
	cmd = exec.Command("cp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	error = cmd.Run()
	if error != nil {
		fmt.Printf("Error while copying docker executable: %v\n", error)
		if _, ok := error.(*exec.ExitError); ok {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		fmt.Printf("Err: %v", error)
		os.Exit(1)
	}*/
}

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	//fmt.Println("Logs from your program will appear here!")

	imageToPull := os.Args[2]
	if !strings.Contains(imageToPull, "/") {
		imageToPull = "library/" + imageToPull
	}
	authToken := getAuthToken(imageToPull)
	layers := pullManifest(imageToPull, authToken)

	 command := os.Args[3]
	 args := os.Args[4:len(os.Args)]
	 file := "/tmp/jail"
	 pullLayers(file, layers, imageToPull, authToken, command)
	 cmd := exec.Command(command, args...)

	 cmd.Stdin = nullReader{}
	 cmd.Stdout = os.Stdout
	 cmd.Stderr = os.Stderr
	 cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: file,
	 	Cloneflags: syscall.CLONE_NEWPID}
	 error := cmd.Run()
	 if error != nil {
		fmt.Printf("Error in waiting on process: %v\n", error)
		if _, ok := error.(*exec.ExitError); ok {
			os.Exit(cmd.ProcessState.ExitCode())
		}
	 	fmt.Printf("Err %v", error)
	 	os.Exit(1)
	 }
	
	os.Exit(cmd.ProcessState.ExitCode())
}
