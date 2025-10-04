<?php
include "guard.php";

$username = $_SESSION["name"];
?>
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <title>RWRiter Chat</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Source+Code+Pro:ital,wght@0,200..900;1,200..900&display=swap"
        rel="stylesheet">
</head>

<body>
    <div class="sidebar">
        <h2><?php echo "Hello, " . $username; ?></h2>
        <hr>
        <h2>Chats</h2>
        <div id="chatList" class="chat-list"></div>
        <br>
        <a href="logout.php"><button>Logout</button></a>
        <br>
        <div class="new-chat">
            <button onclick="newChat()">+ New Chat</button>
        </div>
    </div>
    <div class="main">
        <div id="status" class="status">Connecting to RWRiter...</div>
        <div id="messages" class="messages"></div>
        <div class="input-box">
            <textarea id="prompt" placeholder="Type your message..."></textarea>
            <button onclick="sendMessage()">Send</button>
        </div>
    </div>

    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: "Source Code Pro", monospace;
            margin: 0;
            display: flex;
            height: 100vh;
            background-color: #1a1a1a;
            color: #00FF00;
        }

        button {
            font-family: "Source Code Pro", monospace;
            padding: 0 0.75rem;
            margin: 5px;
            color: white;
            background-color: #333;
            border: 1px solid #000;
            border-radius: 6px;
            transition: background-color 0.3s ease;
        }

        button:hover {
            background-color: #444;
        }

        .sidebar {
            width: 250px;
            background: #222;
            color: #fff;
            display: flex;
            flex-direction: column;
            overflow: auto;
        }

        .sidebar h2 {
            margin: 0;
            padding: 1rem;
            text-align: center;
            background: #222;
            color: #00FF00;
        }

        .chat-list {
            flex: 1;
            overflow-y: auto;
            padding: 0.5rem;
        }

        .chat-list button {
            display: block;
            width: 100%;
            padding: 0.75rem;
            border: 1px solid #00FF00;
            background: #333;
            color: #fff;
            margin-bottom: 0.5rem;
            text-align: left;
            cursor: pointer;
            border-radius: 6px;
        }

        .chat-list button.active {
            background: #555;
        }

        .new-chat {
            padding: 1rem;
            border-top: 1px solid #444;
        }

        .main {
            flex: 1;
            display: flex;
            flex-direction: column;
        }

        .messages {
            flex: 1;
            padding: 1rem;
            overflow-y: auto;
            background: #222;
        }

        .message {
            margin-bottom: 1rem;
            max-width: 97%;
            padding: 0.75rem 1rem;
            border-radius: 8px;
            white-space: pre-wrap;
        }

        .user {
            background: #e0e0e0;
            color: #333;
            align-self: flex-end;
        }

        .bot {
            background: #444;
            color: #fff;
            border: 1px solid #00FF00;
            align-self: flex-start;
        }

        .input-box {
            display: flex;
            padding: 1rem;
            background: #333;
            border-top: 1px solid #444;
        }

        .input-box textarea {
            flex: 1;
            padding: 0.75rem;
            border-radius: 6px;
            border: 1px solid #444;
            resize: none;
            height: 50px;
            background-color: #222;
            color: #00FF00;
            font-family: "Source Code Pro", monospace;
        }

        .input-box button {
            border: none;
            border-radius: 6px;
            cursor: pointer;
            background-color: #00FF00;
            color: black;
            padding: 0.75rem;
        }

        .status {
            padding: 0.5rem 1rem;
            background: #111;
            color: #0f0;
            text-align: center;
        }

        /* Action status styles */
        .action-status {
            background: #2a2a2a;
            border: 1px solid #ffa500;
            color: #ffa500;
            display: flex;
            align-items: center;
            gap: 10px;
            font-style: italic;
            animation: fadeIn 0.3s ease-in;
        }

        /* Spinner animation */
        .spinner {
            width: 16px;
            height: 16px;
            border: 2px solid #444;
            border-top: 2px solid #00FF00;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            display: inline-block;
            margin-left: 8px;
        }

        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        @keyframes fadeIn {
            from {
                opacity: 0;
                transform: translateY(-10px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        code {
            background-color: #333;
            padding: 2px 4px;
            border-radius: 3px;
            font-family: "Source Code Pro", monospace;
        }

        pre {
            background-color: #333;
            padding: 10px;
            border-radius: 6px;
            overflow-x: auto;
            border: 1px solid #555;
        }

        pre code {
            background: none;
            padding: 0;
        }
    </style>

    <script>
    const API_BASE = 'proxy.php?path=';
    let chats = {};
    let currentChat = null;
    let username = <?php echo json_encode($username); ?>;
    let currentActionDiv = null;
    let isProcessing = false;
    let pendingAction = null;

    // ----------------- Backend check -----------------
    async function checkBackend() {
        try {
            let res = await fetch(`${API_BASE}healthz`);
            let data = await res.json();
            document.getElementById("status").textContent = data.ok ? "RWRiter UP" : "RWRiter DOWN";
            document.getElementById("status").style.color = data.ok ? "#00FF00" : "red";
        } catch (e) {
            document.getElementById("status").textContent = "RWRiter DOWN";
            document.getElementById("status").style.color = "red";
        }
    }

    // ----------------- Chat management -----------------
    async function loadChatsFromServer() {
        try {
            let res = await fetch(`${API_BASE}chats/${username}`);
            let data = await res.json();
            data.chats.forEach(c => chats[c] = []);
            if (data.chats.length) {
                currentChat = data.chats[data.chats.length - 1];
                await loadChatHistory(currentChat);
            } else {
                await newChat();
            }
            renderChatList();
        } catch (e) { console.error("Failed to load chats", e); }
    }

    function renderChatList() {
        const list = document.getElementById("chatList");
        list.innerHTML = "";
        Object.keys(chats).forEach(c => {
            const container = document.createElement("div");
            container.style.display = "flex";
            container.style.alignItems = "center";
            container.style.marginBottom = "0.25rem";

            let btn = document.createElement("button");
            btn.textContent = c;
            btn.className = (c === currentChat) ? "active" : "";
            btn.onclick = () => switchChat(c);

            let delBtn = document.createElement("button");
            delBtn.textContent = "ｘ";
            delBtn.style.marginLeft = "0.25rem";
            delBtn.style.width = "40px";
            delBtn.style.height = "40px";
            delBtn.style.border = "1px solid #FF0000";
            delBtn.style.flexShrink = 0;
            delBtn.onclick = async (e) => {
                e.stopPropagation();
                if (!confirm(`Delete chat "${c}"?`)) return;
                try {
                    let res = await fetch(`${API_BASE}chats/${username}/${c}`, { method: "DELETE" });
                    let data = await res.json();
                    if (data.ok) {
                        delete chats[c];
                        if (currentChat === c) currentChat = Object.keys(chats)[0] || null;
                        renderChatList();
                        renderMessages();
                    } else {
                        alert("Failed to delete chat: " + (data.error || "Unknown error"));
                    }
                } catch (err) {
                    alert("Error deleting chat: " + err);
                }
            };

            container.appendChild(btn);
            container.appendChild(delBtn);
            list.appendChild(container);
        });
    }

    function switchChat(chatname) {
        currentChat = chatname;
        renderMessages();
        renderChatList();
    }

    async function newChat() {
        try {
            let res = await fetch(`${API_BASE}chats/${username}/new`, { method: "POST" });
            let data = await res.json();
            chats[data.chat_id] = [];
            currentChat = data.chat_id;
            renderChatList();
            renderMessages();
        } catch (e) { console.error("Failed to create chat", e); }
    }

    async function loadChatHistory(chatname) {
        try {
            let res = await fetch(`${API_BASE}session/${username}/${chatname}`);
            let data = await res.json();
            chats[chatname] = data.history || [];
            renderMessages();
        } catch (e) { console.error("Failed to load chat history", e); }
    }

    // ----------------- Message formatting -----------------
    function formatMessageForDisplay(text) {
        let html = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
        html = html.replace(/\*\*(.+?)\*\*/g, '<b>$1</b>')
                   .replace(/\*(.+?)\*/g, '<i>$1</i>')
                   .replace(/`(.+?)`/g, '<code>$1</code>')
                   .replace(/```([\s\S]+?)```/g, '<pre><code>$1</code></pre>');
        return html;
    }

    function renderMessages() {
        const box = document.getElementById("messages");
        box.innerHTML = "";
        if (!currentChat || !chats[currentChat]) return;
        chats[currentChat].forEach(msg => {
            let div = document.createElement("div");
            div.className = "message " + (msg.role === "user" ? "user" : "bot");
            if (msg.role === "user") {
                div.textContent = `[${username}] ${msg.content}`;
            } else {
                div.innerHTML = `[RWRiter] ` + formatMessageForDisplay(msg.content);
            }
            box.appendChild(div);
        });
        box.scrollTop = box.scrollHeight;
    }

    // ----------------- Action status helpers -----------------
    function showSpinner() {
        const spinnerDiv = document.createElement("div");
        spinnerDiv.className = "message bot action-status";
        spinnerDiv.innerHTML = `[RWRiter] <div class="spinner"></div>`;
        document.getElementById("messages").appendChild(spinnerDiv);
        spinnerDiv.scrollIntoView({ behavior: "smooth" });
        return spinnerDiv;
    }

    function hideSpinner(spinnerDiv) {
        if (spinnerDiv) {
            spinnerDiv.remove();
        }
    }

    // ----------------- Message sending -----------------
    async function sendMessage() {
        const promptEl = document.getElementById("prompt");
        let prompt = promptEl.value.trim();
        if (!prompt || !currentChat || isProcessing) return;

        promptEl.value = "";
        isProcessing = true;

        chats[currentChat].push({ role: "user", content: prompt });
        renderMessages();

        try {
            const res = await fetch(`${API_BASE}chat/${username}/${currentChat}`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ prompt, stream: true })
            });

            const reader = res.body.getReader();
            const decoder = new TextDecoder();
            let finalContent = "";

            // create streaming bot message container
            const botDiv = document.createElement("div");
            botDiv.className = "message bot";
            botDiv.innerHTML = "[RWRiter] ";
            document.getElementById("messages").appendChild(botDiv);
            botDiv.scrollIntoView({ behavior: "smooth" });

            while (true) {
                const { done, value } = await reader.read();
                if (done) break;

                let chunk = decoder.decode(value, { stream: true });

                // Directly append streamed text; no filtering
                finalContent += chunk;
                botDiv.innerHTML = "[RWRiter] " + formatMessageForDisplay(finalContent);
                botDiv.scrollIntoView({ behavior: "smooth" });
            }

            // finalize
            botDiv.innerHTML = "[RWRiter] " + formatMessageForDisplay(finalContent.trim());

            chats[currentChat].push({ role: "bot", content: finalContent.trim() });

        } catch (e) {
            console.error("Error in sendMessage:", e);
            const errorDiv = document.createElement("div");
            errorDiv.className = "message bot";
            errorDiv.innerHTML = "[RWRiter] Error contacting RWRiter";
            document.getElementById("messages").appendChild(errorDiv);
            chats[currentChat].push({ role: "bot", content: "Error contacting RWRiter" });
        } finally {
            isProcessing = false;
        }
    }

    // ----------------- Allow Enter to send -----------------
    document.getElementById("prompt").addEventListener("keydown", function (e) {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    });

    // ----------------- Init -----------------
    checkBackend();
    loadChatsFromServer();
    </script>

</body>
</html>
