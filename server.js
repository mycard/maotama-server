'use strict';
const WebSocketServer = require("websocket").server;
const https = require("https");
const dgram = require("dgram");
const fs = require('fs');

const address = process.env['ADDRESS'];
if (!address) {
    throw "Set IP address in environment variable ADDRESS"
}

const options = {
    key: fs.readFileSync('cert/privkey.pem'),
    cert: fs.readFileSync('cert/fullchain.pem')
};

let http_server = https.createServer(options, (request, response) => {
    response.writeHead(400);
    response.end()
}).listen(10800);

let websocket_server = new WebSocketServer({httpServer: http_server, autoAcceptConnections: true});

websocket_server.on("connect", (connection) => {
    console.log(`${new Date()} Connection accepted.`);
    let server = dgram.createSocket('udp4');
    let clients = new Map(); //<-- server.ip:address -> client -->
    let alive = false;
    server.bind({}, () => {
        connection.send(`LISTEN ${address}:${server.address().port}`);
    });
    server.on('message', (message, client_remote)=> {
        let client_id = `${client_remote.address}:${client_remote.port}`;
        let client = clients.get(client_id);
        if (client) {
            // 已经创建了 client
            if (client.remote) {
                // 已经打洞已成功，转发之
                client.send(message, client.remote.port, client.remote.address);
            } else {
                // 还没没打洞成功，将报文存起来等打洞成功后转发给客户端。
                client.buffers.push(message);
            }
        } else {
            // 还没创建 client (第一次从这个地址收到消息)
            client = dgram.createSocket('udp4');
            clients.set(client_id, client);
            client.buffers = [message];
            client.bind({}, ()=> {
                connection.send(`CONNECT ${address}:${client.address().port}`);
            });
            client.on('message', (message, server_remote)=> {
                alive = true;
                if (client.remote) {
                    // 已经打洞成功，这个是主机发来要转发给客户端的报文
                    server.send(message, client_remote.port, client_remote.address)
                } else {
                    // 刚刚打洞成功，这个是打洞报文
                    connection.send(`CONNECTED ${address}:${client.address().port}`);
                    client.remote = server_remote;
                    for (let message of client.buffers) {
                        client.send(message, server_remote.port, server_remote.address);
                    }
                    client.buffers = null;
                }
            })
        }
    });
    // 如果 websocket 连接已经关闭，并且2分钟没有通讯，就释放跟这个 websocket 连接相关的资源
    connection.on('close', () => {
        console.log('websocket connection close');
        alive = true;
        let id = setInterval(()=> {
            if (alive) {
                alive = false
            } else {
                console.log('close server and client udp');
                server.close();
                for (let [id, client] of clients) {
                    client.close();
                }
                clearInterval(id);
            }
        }, 1200);
    })
});
