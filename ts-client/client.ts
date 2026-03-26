// messed up method to ensure socket is connected before we send messages
function ensureConnection(socket: WebSocket): Promise<void> {
    return new Promise((resolve, reject) => {
        socket.onopen = () => {
            resolve();
        };
        
        socket.onerror = () => {
            reject();
        }

        socket.onclose = () => {
            reject();
        }
    });
}

// "main" function
(async () => {
    // Establish initial connection
    const socket = new WebSocket("ws://rP2gIxhkw7xHVpwGOX6g.cloudramp.org:8080/awkdhawdw");
    await ensureConnection(socket);

    // Receive the first message then close
    socket.onmessage = (ev: MessageEvent) => {
        console.log("Got message from server:", ev.data);
    }

    // Send initial message once everything is ready
    // socket.send("Hello server from TypeScript!");
})()