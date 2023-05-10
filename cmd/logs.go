package cmd

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type LogListItem struct {
	Filename    string
	Size        int64
	PrintedSize string
}
type LogList struct {
	Success     bool
	Error       string
	SessionName string
	List        []LogListItem
}
type LogPart struct {
	Success bool
	Error   string
	Lines   []string
}

func MBToGB(b uint64) (float64, int) {
	const unit = 1024
	if b < unit {
		return float64(b), 0
	}

	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return float64(b) / float64(div), exp
}

func ByteCount(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	bGB, exp := MBToGB(b)
	return fmt.Sprintf("%.1f%cB", bGB, "KMGTPE"[exp])
}

// processLogList produces the list of available lots inside the div element (into the writer w), using log_list.html template and LogList object.
func processLogList(w http.ResponseWriter, templ *template.Template, success bool, sessionName string, result string) {
	var ll = LogList{SessionName: sessionName}
	ll.processResponse(result, success)
	if err := templ.ExecuteTemplate(w, "log_list.html", ll); err != nil {
		fmt.Fprintf(w, "Failed executing log_list template: %v", err)
		return
	}
}

func (ll *LogList) processResponse(result string, success bool) {
	if !success {
		ll.Error = result
		return
	}

	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		ll.Error = fmt.Sprintf("incorrect response (length of lines should be at least 2): %v", lines)
		return
	}
	if !strings.HasPrefix(lines[0], successLine) {
		ll.Error = fmt.Sprintf("incorrect response (first line needs to be SUCCESS): %v", lines)
		return
	}

	for _, l := range lines[1:] {
		if len(l) == 0 {
			continue
		}

		terms := strings.Split(l, " | ")
		if len(terms) != 2 {
			ll.Error = fmt.Sprintf("incorrect response line (need to have 2 terms divided by |): %v", l)
			return
		}

		size, err := strconv.ParseUint(terms[1], 10, 64)
		if err != nil {
			ll.Error = fmt.Sprintf("incorrect size: %v", terms[1])
			return
		}
		ll.List = append(ll.List, LogListItem{Filename: terms[0], Size: int64(size), PrintedSize: ByteCount(size)})
	}
	ll.Success = true
}

// Produces (into writer w) log part (head or tail) inside the div HTML element, using log_read.html template and LogPart object
func processLogPart(w http.ResponseWriter, templ *template.Template, success bool, sessionName string, result string) {
	var lp LogPart
	lp.processResponse(result, success)
	if err := templ.ExecuteTemplate(w, "log_read.html", lp); err != nil {
		fmt.Fprintf(w, "Failed executing log_read template: %v", err)
		return
	}
}

func (lp *LogPart) processResponse(result string, success bool) {
	if !success {
		lp.Success = false
		lp.Error = result
		return
	}
	lp.Success = true

	lines := strings.Split(result, "\n")
	if len(lines) >= 1 && strings.HasPrefix(lines[0], successLine) {
		lines = lines[1:]
	}
	lp.Lines = lines
}

var logReadFirstLine = regexp.MustCompile("^SUCCESS: ([0-9]+)-([0-9]+)/([0-9]+)$")

// parseLogPart parses the response from the erigon node, which contains a part of a log file.
// It should start with a line of format: SUCCESS from_offset/to_offset/total_size,
// followed by the actual log chunk.
func parseLogPart(nodeRequest *NodeRequest, offset uint64) (bool, uint64, uint64, []byte, string) {
	nodeRequest.lock.Lock()
	defer nodeRequest.lock.Unlock()
	if !nodeRequest.served {
		return false, 0, 0, nil, ""
	}
	clear := nodeRequest.retries >= 16
	if nodeRequest.err != "" {
		return clear, 0, 0, nil, nodeRequest.err
	}
	firstLineEnd := bytes.IndexByte(nodeRequest.response, '\n')
	if firstLineEnd == -1 {
		return clear, 0, 0, nil, "could not find first line in log part response"
	}
	m := logReadFirstLine.FindSubmatch(nodeRequest.response[:firstLineEnd])
	if m == nil {
		return clear, 0, 0, nil, fmt.Sprintf("first line needs to have format SUCCESS: from-to/total, was [%sn", nodeRequest.response[:firstLineEnd])
	}
	from, err := strconv.ParseUint(string(m[1]), 10, 64)
	if err != nil {
		return clear, 0, 0, nil, fmt.Sprintf("parsing from: %v", err)
	}
	if from != offset {
		return clear, 0, 0, nil, fmt.Sprintf("Unexpected from %d, wanted %d", from, offset)
	}
	to, err := strconv.ParseUint(string(m[2]), 10, 64)
	if err != nil {
		return clear, 0, 0, nil, fmt.Sprintf("parsing to: %v", err)
	}
	total, err := strconv.ParseUint(string(m[3]), 10, 64)
	if err != nil {
		return clear, 0, 0, nil, fmt.Sprintf("parsing total: %v", err)
	}
	return true, to, total, nodeRequest.response[firstLineEnd+1:], ""
}

// LogReader implements io.ReaderSeeker to be used as parameter to http.ServeContent.
type LogReader struct {
	filename       string // Name of the log files to download
	requestChannel chan *NodeRequest
	total          uint64 // Size of the log file to be downloaded. Needs to be known before download
	offset         uint64 // Current offset set either by the Seek() or Read() functions
	ctx            context.Context
}

// Read is part of the io.Reader interface - emulates reading from the remote logs as if it was from the web server itself.
func (lr *LogReader) Read(p []byte) (n int, err error) {
	nodeRequest := &NodeRequest{url: fmt.Sprintf("/logs/read?file=%s&offset=%d\n", url.QueryEscape(lr.filename), lr.offset)}
	lr.requestChannel <- nodeRequest
	var total uint64
	var clear bool
	var part []byte
	var errStr string
	for nodeRequest != nil {
		select {
		case <-lr.ctx.Done():
			return 0, fmt.Errorf("interrupted")
		default:
		}
		clear, _, total, part, errStr = parseLogPart(nodeRequest, lr.offset)
		if clear {
			nodeRequest = nil
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if errStr != "" {
		return 0, fmt.Errorf(errStr)
	}
	lr.total = total
	copied := copy(p, part)
	lr.offset += uint64(copied)
	if lr.offset == total {
		return copied, io.EOF
	}
	return copied, nil
}

// Part of the io.Seeker interface. Please note io.SeekEnd - this is used by http.ServeContent to establish content length
func (lr *LogReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		lr.offset = uint64(offset)
	case io.SeekCurrent:
		lr.offset = uint64(int64(lr.offset) + offset)
	case io.SeekEnd:
		if lr.total > 0 {
			lr.offset = uint64(int64(lr.total) + offset)
		} else {
			lr.offset = 0
		}
	}
	return int64(lr.offset), nil
}

// Handles the use case when operator clicks on the link with the log file name, and this initiates the download of this file
// to the operator's computer (via browser). See LogReader above which is used in http.ServeContent
func transmitLogFile(ctx context.Context, r *http.Request, w http.ResponseWriter, sessionName string, filename string, size uint64, requestChannel chan *NodeRequest) {
	if requestChannel == nil {
		fmt.Fprintf(w, "ERROR: Node is not allocated\n")
		return
	}
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": sessionName + "_" + filename})
	w.Header().Set("Content-Disposition", cd)
	w.Header().Set("Content-Type", "application/octet-stream")
	logReader := &LogReader{filename: filename, requestChannel: requestChannel, offset: 0, total: size, ctx: ctx}
	http.ServeContent(w, r, filename, time.Now(), logReader)
}
