* autoindex http server with file renaming

we never actually auth to the destiny.gg chat... there was no auth code and this was never used

```go
func (c *DestinyChat) send(command string, msg interface{}) error {
  data, err := json.Marshal(msg)
  if err != nil {
    return err
  }

  buf := bytes.NewBuffer([]byte{})
  buf.WriteString(command)
  buf.WriteString(" ")
  buf.Write(data)

  c.Lock()
  defer c.Unlock()
  if err := c.conn.WriteMessage(1, buf.Bytes()); err != nil {
    log.Println("error sending message %s", err)
    c.reconnect()
    return err
  }

  return nil
}

// Write send message
func (c *DestinyChat) Write(command, data string) {
  c.send(command, struct{ data string }{data})
}

// WritePrivate send private message
func (c *DestinyChat) WritePrivate(command, nick, data string) {
  c.send(command, struct {
    nick string
    data string
  }{nick, data})
}
```