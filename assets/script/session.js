async function fetchContent(sessionName, description, url, divId) {
    try {
        const d = document.getElementById(divId);
        d.innerHTML = `Fetching ${description} ...`;

        const formData = new FormData();
        formData.append('current_session_name', sessionName);

        const request = createRequest(url, "POST", formData);
        const response = await fetch(request);

        if (!response.ok) {
            d.innerHTML = "ERROR: Network response was not OK";
            return;
        }

        d.innerHTML = await response.text();
    } catch (error) {
        const d = document.getElementById(divId);
        d.innerHTML = `ERROR: ${error.message}`;
    }
}

async function fetchLogPart(logPartId, sessionName, description, url, filename, size) {
    try {
        const d = document.getElementById(logPartId);
        d.innerHTML = `Fetching ${description} ...`;

        const formData = createForm({
            'current_session_name': sessionName,
            'file': filename,
            'size': size,
        });

        const request = createRequest(url, "POST", formData);
        const response = await fetch(request);

        if (!response.ok) {
            d.innerHTML = "ERROR: Network response was not OK";
            return;
        }

        d.innerHTML = await response.text();
    } catch (error) {
        const d = document.getElementById(logPartId);
        d.innerHTML = `ERROR: ${error.message}`;
    }
}

async function clearLog(logPartId) {
    const d = document.getElementById(logPartId);
    d.innerHTML = ""
}

async function processReader(d, reader) {
    let first = true
    const utf8Decoder = new TextDecoder("utf-8");
    let buffer = ''
    for (let chunk = await reader.read();!chunk.done;chunk = await reader.read()) {
        if (!chunk.done) {
            buffer += utf8Decoder.decode(chunk.value, { stream: false });
        }
        const lastLineBreak = buffer.lastIndexOf('\n');
        if (lastLineBreak !== -1) {
            if (first) {
                d.innerHTML = buffer.substring(0, lastLineBreak);
                first = false
            } else {
                d.innerHTML += buffer.substring(0, lastLineBreak);
            }
            buffer = buffer.substring(lastLineBreak + 1)
        }
    }
    if (first) {
        d.innerHTML = buffer;
        first = false;
    } else {
        d.innerHTML += buffer;
    }
}

async function findReorgs(sessionName) {
    const d = document.getElementById('reorgs');
    d.innerHTML = "Looking for reorgs...";
    let formData = new FormData();
    formData.append('current_session_name', sessionName);
    const request =  createRequest("/ui/reorgs", "POST", formData)
    fetch(request)
        .then((response) => response.body)
        .then((body) => body.getReader())
        .then((reader) => processReader(d, reader))
        .then(() => console.log('completed'))
        .catch((err) => d.innerHTML = "ERROR: " + err.message);
}

async function findReorgsInDetail(sessionName) {
    const d = document.getElementById('reorgs_in_detail');
    d.innerHTML = "Looking for reorgs...";
    let formData = new FormData();
    formData.append('current_session_name', sessionName);
    const request =  createRequest("/ui/reorgs_in_detail", "POST", formData)
    fetch(request)
        .then((response) => response.body)
        .then((body) => body.getReader())
        .then((reader) => processReader(d, reader))
        .then(() => console.log('completed'))
        .catch((err) => d.innerHTML = "ERROR: " + err.message);
}


async function processReaderReplace(d, reader) {
    const utf8Decoder = new TextDecoder("utf-8");
    let buffer = ''
    let lines;
    for (let chunk = await reader.read(); !chunk.done; chunk = await reader.read()) {
        if (!chunk.done) {
            buffer += utf8Decoder.decode(chunk.value, {stream: false});
        }
        let lastLineBreak = buffer.lastIndexOf('\n')
        if (lastLineBreak !== -1) {
            lines = buffer.substring(0, lastLineBreak).split('\n')
            buffer = buffer.substring(lastLineBreak + 1)
            d.innerHTML = buffer.substring(0, lastLineBreak);
            lines.forEach(line => {
                d.innerHTML = line;
            });
        }
    }
    d.innerHTML = buffer;
}

async function bodiesDownload(sessionName) {
    const d = document.getElementById('bodies_download');
    d.innerHTML = "Tracking bodies download...";
    let formData = new FormData();
    formData.append('current_session_name', sessionName);
    const request =  createRequest("/ui/bodies_download", "POST", formData)
    fetch(request)
        .then((response) => response.body)
        .then((body) => body.getReader())
        .then((reader) => processReaderReplace(d, reader))
        .then(() => console.log('completed'))
        .catch((err) => d.innerHTML = "ERROR: " + err.message);
}

async function headersDownload(sessionName) {
    const d = document.getElementById('headers_download');
    d.innerHTML = "Tracking headers download...";
    var formData = new FormData();
    formData.append('current_sessionname', sessionName);
    const request =  createRequest("/ui/headers_download", "POST", formData) 
    fetch(request)
        .then((response) => response.body)
        .then((body) => body.getReader())
        .then((reader) => processReaderReplace(d, reader))
        .then(() => console.log('completed'))
        .catch((err) => d.innerHTML = "ERROR: " + err.message);
}

async function fetchSession(sessionName, description, sessionPin) {
    let formData = null
    let url
    if(description === 'switch'){
        let current_session_name = document.getElementById("current_session_name").value
        formData = createForm({
            "current_session_name": current_session_name,
            "session_name": sessionName,
            "pin": "",
            [sessionPin]: sessionName
        })
        url = "/ui/switch"
    }
    else if(description === 'resume'){
        let newSessionName = document.getElementById("session_name").value
        let newPin = document.getElementById("pin").value
        formData = createForm({
            "current_session_name": sessionName,
            "session_name": newSessionName,
            "resume_session": "Resume or take over operator session",
            "pin": newPin
        })
        url = "/ui/resume"
    }
    const request =  createRequest(url, "POST", formData)
    fetch(request)
        .then(response => response.text())
        .then(html => {
            document.body.innerHTML = html;
        })
}

function createForm(jsonData) {
    let formData = new FormData();
    for (const [key, value] of Object.entries(jsonData)) {
        formData.append(key, value)
    }
    return formData;
}

function createRequest(url, method, formData) {
    return  new Request(url, {
        method: method,
        mode: "cors",
        cache: "default",
        body: formData,
    })
}