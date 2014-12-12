
rpc = require 'framed-msgpack-rpc'
assert = require 'assert'

x = rpc.createTransport { host: '127.0.0.1', port : 8222 }
await x.connect defer err
if err
  console.log "error connecting"
else
    c = new rpc.Client x, "test.1"
    await c.invoke 'arith.Add', { a : 5, b : 4}, defer err, response
    if err? then console.log "error in RPC: #{err}"
    else assert.equal 9, response.c
    x.close()
    console.log "all good!"
process.exit 0
