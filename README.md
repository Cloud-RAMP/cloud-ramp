# Cloud ramp

Official repo for official business.

### Architecture

The main driver in this application is the [wasm-sandbox](https://github.com/Cloud-RAMP/wasm-sandbox), which takes user code and executes it in a WASM-based runtime. This repo is merely a fancy wrapper for that.

#### Connection Establishment

The main entrypoint is the WebSocket server. When a new connection is created, a few things will happen:
* The connection is registered with the rate limiter (based on IP)
  * If any rate limiting data for this connection existed on Redis and had not expired yet, it will be pulled to the local server instance for the duration of the connection
  * If the number of requests is higher than the limit, send a backoff message
* Upgrade the HTTP connection to WebSocket
* Check if the userID for the given IP existed previously
  * This is done to support connection recovery. For 30s after a connection is cancelled, the ip:userID pair will be in Redis. If the same IP connects within that window, they are assigned the same userID
  * If no user ID is found, create a new one for the connection
* Initialize a new "comm channel" for the connection
  * This is a set of channels on the local server instance that connections can use to communicate
  * This is done if two connections on the same machine want to communicate without using Redis
* Subscribe a listener to the Redis database for event reception
* Add the user's ID to a redis set that represents all current users within a room
* Send a message to other users in the room notifying them of the join event
* Execute the user's `onJoin` method
* Spin off a goroutine to listen for messages from Redis and the comm channel
  * If messages are received, send over the WebSocket connection
* In the main goroutine, listen for messages from the client and call the user's `onMessage` function

#### Connection Close

* Execute the user's `onLeave` method
* Send message to users in the room notifying them of the leave event
* Send the current local rate limiter count to Redis
  * Include TTL based on the last request
* Remove the user from the Redis set representing room membership
  * If the user was the last one in the set, remove the entire key
* Add the `ip:connection-id` pair to Redis with a TTL of 30 seconds, so if the client reconnects again their old connection is established
* Close the "comm" channel to communicate between connections on the same machine
  * Similarly, if this was the last member of the room, delete it
* Close the connection

#### System start

* Load environment variables
  * Redis URL, Firestore URL, Vercel storage URL
* Create a signal handler to perform actions on shutdonw
* Initialize the firestore client, sandbox, and redis
* Start the server

#### System shutdown

* Shutdown the server, waiting for all connections to complete
  * Limited to a 15s timeout. If not completed by then, connections will be closed forcefully
* Dump all local logs to firestore
* Dump all rate limiter data to Redis 

#### Logging

There are two types of logging in this system. Instance and server logging.
* Instance logging
  * These are per-instance logs that will be sent to the user
  * Data relevant to each instance is batched in the local filesystem
  * These files are sent to Firestore on a timeout
  * This is done to avoid high Firestore writes, as we have a limited quota
  * These logs will be seen on our site, [cloudramp.org](cloudramp.org)
* Local logging
  * This is done directly in the terminal, and applies to functions run on the server that don't concern the user
  * Eg. networking failures (abnormal connection, failed redis message), new connection establishment, services starting / stopping
  * These logs will only be seen by the developers in our deployed service

### Local Development

#### Setup

1. Make sure you have go 1.24 downloaded (or some future version)
2. To make sure it works with our custom URL, add the following line to `/etc/hosts` (using `sudo nano /etc/hosts/`):
```
127.0.0.1   backend.cloudramp.org
```

> DNS checks first examine the `/etc/hosts` file before performing a lookup, meaning that these will all resolve to localhost

#### Running

1. cd into the project directory, and run `make run` in one terminal to start the server
2. In another terminal, run `make run-client` to run a simple client that connects to the server
3. If you want to run it interactively, follow these steps:
   1. Run `make run` in the original cloud ramp directory
   2. Create a new terminal and cd into the `cloud-ramp-frontend` directory, run `npm i` if packages are not installed, and run `npm run dev`
   3. This will start the local server, where our cloudramp site is now accessible on `localhost:3000`
   4. Upload a new WASM module
   5. Click on the module and open the playground, here you will be able to join a room and send messages over the connection

> The local development process is meant to only simulate one server replication. Since this is what will be hosted on the edge, if one process works, any number of them _should_ work.