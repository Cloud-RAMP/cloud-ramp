package cfg

import "time"

// package to keep simple config variables.
//
// in a production system, we would probably load config with a yaml file.
// for simplicity, we can just define variables here

const USE_FIRESTORE = false

// How often logs for each instance will be dumped to firestore.
//
// Increase the size = fewer dumps, more storage used on container,
// fewer operations used on Firestore
//
// Be careful that our container may be preempted at any time, so having a higher cooldown
// can mean losing logs
const LOG_DUMP_INTERVAL_SECONDS = 12000000

// If this value is true, we use the loader that pulls from the local filesystem
//
// Else, we will pull WASM modules from vercel blob storage
const USE_MOCK_LOADER = true

// If set to true, rate limiting will be enforced
const RATE_LIMIT = true

// If an IP surpasses MAX_REQEUSTS_PER_WINDOW in RATE_LIMIT_WINDOW_SECONDS,
// they will be backed off.
const RATE_LIMIT_WINDOW_SECONDS = 30
const MAX_REQUESTS_PER_WINDOW = 1000

// How often we dump local rates to Redis
//
// We can assume that most IPs will only connect to one server, unless they know their stuff
const RATE_DUMP_INTERVAL_SECONDS = 10

const IP_RECONNECT_TTL_SECONDS = 30

// time.Duration objects from config varaibles. Not necessary to modify
const RATE_TTL = time.Duration(RATE_LIMIT_WINDOW_SECONDS * time.Second)
const RATE_DUMP_INTERVAL = time.Duration(RATE_DUMP_INTERVAL_SECONDS * time.Second)
const IP_RECONNECT_TTL = time.Duration(IP_RECONNECT_TTL_SECONDS * time.Second)
