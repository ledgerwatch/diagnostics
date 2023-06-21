package erigon_node

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ledgerwatch/diagnostics/internal"
)

var _ Client = &NodeClient{}

type NodeClient struct {
	versions *Versions
}

func (c *NodeClient) Version(_ context.Context, requestChannel chan *internal.NodeRequest) (Versions, error) {
	success, result := c.fetch("/version\n", requestChannel)
	var versions Versions

	if success {
		lines := strings.Split(result, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], SuccessLine) {
			versions.Success = true
			if len(lines) < 2 {
				versions.Error = "at least node version needs to be present"
				versions.Success = false
			} else {
				var err error
				versions.NodeVersion, err = strconv.ParseUint(lines[1], 10, 64)
				if err != nil {
					versions.Error = fmt.Sprintf("parsing node version [%s]: %v", lines[1], err)
					versions.Success = false
				} else {
					for idx, line := range lines[2:] {
						switch idx {
						case 0:
							versions.CodeVersion = line
						case 1:
							versions.GitCommit = line
						}
					}
				}
			}
		} else {
			versions.Error = fmt.Sprintf("incorrect response (first line needs to be SUCCESS): %v", lines)
		}
		c.versions = &versions
	} else {
		versions.Error = result
	}

	return versions, nil
}

func (c *NodeClient) Flags(ctx context.Context, requestChannel chan *internal.NodeRequest) (Flags, error) {
	// If Versions have not been retrieved, go ahead and retrieve them
	if c.versions == nil {
		_, err := c.Version(ctx, requestChannel)
		if err != nil {
			//fmt.Fprintf(w, "Unable to process flag due to inability to get node version: %s", versions.Error)
			return Flags{}, err
		}
	}

	if c.versions.NodeVersion < 2 {
		//fmt.Fprintf(w, "Flags only support version >= 2. Node version: %d", versions.NodeVersion)
		// TODO semeantic error propagate
		return Flags{}, nil
	}

	// Retrieving the data from the node
	success, result := c.fetch("/flags\n", requestChannel)

	var flags Flags
	flags.FlagPayload = make(map[string]string)
	if success {
		lines := strings.Split(result, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], SuccessLine) {
			flags.Success = true
			for _, line := range lines[1:] {
				if len(line) > 0 {
					flagName, flagValue, found := strings.Cut(line, "=")
					if !found {
						flags.Error = fmt.Sprintf("fail to parse line %s", line)
						flags.Success = false
					} else {
						flags.FlagPayload[flagName] = flagValue
					}
				}
			}
		} else {
			flags.Error = fmt.Sprintf("incorrect response (first line needs to be SUCCESS): %v", lines)
		}
	} else {
		flags.Error = result
	}

	return flags, nil
}

func (c *NodeClient) CMDLineArgs(_ context.Context, requestChannel chan *internal.NodeRequest) CmdLineArgs {
	success, result := c.fetch("/cmdline\n", requestChannel)
	var args CmdLineArgs
	if success {
		if strings.HasPrefix(result, SuccessLine) {
			args.Args = strings.ReplaceAll(result[len(SuccessLine):], "\n", " ")
		} else {
			args.Args = result
		}
		args.Success = true
	} else {
		args.Success = false
		args.Error = result
	}

	return args
}

func (c *NodeClient) fetch(url string, requestChannel chan *internal.NodeRequest) (bool, string) {
	if requestChannel == nil {
		return false, "ERROR: Node is not allocated\n"
	}
	// Request command line arguments
	nodeRequest := &internal.NodeRequest{Url: url}
	requestChannel <- nodeRequest
	var sb strings.Builder
	var success bool
	for nodeRequest != nil {
		nodeRequest.Lock.Lock()
		clear := nodeRequest.Served
		if nodeRequest.Served {
			if nodeRequest.Err == "" {
				sb.Reset()
				sb.Write(nodeRequest.Response)
				success = true
			} else {
				success = false
				fmt.Fprintf(&sb, "ERROR: %s\n", nodeRequest.Err)
				if nodeRequest.Retries < 15 { // TODO: MaxRequestRetries use this  instead of 15
					clear = false
				}
			}
		}
		nodeRequest.Lock.Unlock()
		if clear {
			nodeRequest = nil
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return success, sb.String()
}

func (c *NodeClient) getResultLines(result string) ([]string, error) {
	lines := strings.Split(result, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], SuccessLine) {
		return nil, fmt.Errorf("incorrect response (first line needs to be SUCCESS): %s", result)
	}

	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	return lines[1:], nil
}

func NewErigonNodeClient() Client {
	return &NodeClient{}
}

type Client interface {
	// Version retrieves the versions from the Erigon node exposed
	Version(ctx context.Context, requestChannel chan *internal.NodeRequest) (Versions, error)
	// Flags given version has been retrieved, provides the flags
	Flags(ctx context.Context, requestChannel chan *internal.NodeRequest) (Flags, error)
	// CMDLineArgs retrieves the command line arguments provided to run the erigon node
	CMDLineArgs(ctx context.Context, requestChannel chan *internal.NodeRequest) CmdLineArgs
	// fetch requests the data from the specified end point
	fetch(url string, requestChannel chan *internal.NodeRequest) (bool, string)
	getResultLines(result string) ([]string, error)

	// TODO: refactor the following methods to follow above pattern where appropriate
	FindSyncStages(ctx context.Context, w http.ResponseWriter, template *template.Template, requestChannel chan *internal.NodeRequest)
	BodiesDownload(ctx context.Context, w http.ResponseWriter, template *template.Template, requestChannel chan *internal.NodeRequest)
	HeadersDownload(ctx context.Context, w http.ResponseWriter, template *template.Template, requestChannel chan *internal.NodeRequest)
	FindReorgs(ctx context.Context, w http.ResponseWriter, template *template.Template, requestChannel chan *internal.NodeRequest)
	FindDeepReorgs(ctx context.Context, w http.ResponseWriter, template *template.Template, requestChannel chan *internal.NodeRequest)
	ProcessLogList(w http.ResponseWriter, template *template.Template, sessionName string, requestChannel chan *internal.NodeRequest)
	LogHead(filename string, requestChannel chan *internal.NodeRequest) LogPart
	LogTail(filename string, offset uint64, requestChannel chan *internal.NodeRequest) LogPart
}
