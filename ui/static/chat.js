function Chat(ws) {

    ws.onopen = function () {
        console.log("Opened");
        const path = "/user/" + id;
        const payload = {"action": "subscribe", "path" : path};
        ws.send(JSON.stringify(payload))
    };

    ws.onmessage = function (ev) {
        const json = ev.data;
        console.log(json);

        const data = JSON.parse(json);

        const newMessage = "<b>" + data.from + "</b>: " + data.text;
        appendMessage(newMessage);
    };

    ws.onerror = function (ev) {
        console.log("Error => " + ev)
    };
}

function sendMessage() {

    // get email address of the friend the user
    // is chatting with
    const friendEmail = document.getElementById("friend_email").value;
    if (friendEmail === "") {
        alert("Enter a valid friend Id");
        return
    }

    const path = "/user/" + friendEmail;
    const messageBox = document.getElementById("message_box");
    const text = messageBox.value;

    const message = {"text" : text, "from" : id};
    const payload = {"action" : "message", "data" : message, "path" : path};
    ws.send(JSON.stringify(payload));

    const newMessage = "<b>" + id + "</b>: " + text;
    appendMessage(newMessage);
    // empty message box
    messageBox.value = "";
}

function appendMessage(text) {
    const prevValue = document.getElementById("chat-window").innerHTML;
    const newValue = prevValue + "<br />" + text

    document.getElementById("chat-window").innerHTML = newValue;
}

// get user email from cookie
const id = document.cookie.split("=")[1];
const localWsUrl = "wss://localhost:9005/ws/connect?user=" + id; // use this when testing locally
const remoteWsUrl = "ws://go-chat-example.herokuapp.com/ws/connect?user=" + id;
const ws = new WebSocket(localWsUrl);
Chat(ws);
