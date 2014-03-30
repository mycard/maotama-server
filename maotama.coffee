ip = "117.121.26.153"
originIsAllowed = (origin) ->
  true
WebSocketServer = require("websocket").server
http = require("http")
dgram = require("dgram");
http_server = http.createServer((request, response) ->
  console.log (new Date()) + " Received request for " + request.url
  response.writeHead 404
  response.end()
  return
)
http_server.listen 10800, ->
  console.log (new Date()) + " Server is listening on port 8080"
wsServer = new WebSocketServer(
  httpServer: http_server
  autoAcceptConnections: false
)
wsServer.on "request", (request) ->
  unless originIsAllowed(request.origin)
    # Make sure we only accept requests from an allowed origin
    request.reject()
    console.log (new Date()) + " Connection from origin " + request.origin + " rejected."
    return
  connection = request.accept("shinkirou", request.origin)
  console.log (new Date()) + " Connection accepted."
  server = dgram.createSocket('udp4');
  clients = {}
  server.on 'listening', ()->
    connection.send "LISTEN #{ip}:#{server.address().port}"
  server.on 'message', (message, remote)->
    #console.log 'server', server.address().port, message, remote
    if !clients[remote.port]
      client = clients[remote.port] = dgram.createSocket('udp4');
      client.on 'listening', ()->
        connection.send "PUNCH #{ip}:#{client.address().port}"
      client.on 'message', (message, server_info)->
        #console.log 'client', client.address().port, message, server_info
        if !client.server_info
          connection.send "PUNCHOK #{ip}:#{client.address().port}"
          client.server_info = server_info
        else
          server.send message, 0, message.length, remote.port, remote.address
      client.bind()
    else if clients[remote.port].server_info
      client = clients[remote.port]
      client.send message, 0, message.length, client.server_info.port, client.server_info.address
  server.bind()
  connection.on "close", (reasonCode, description) ->
    console.log (new Date()) + " Peer " + connection.remoteAddress + " disconnected."
