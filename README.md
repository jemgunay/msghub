# msghub
A basic chat room orientated message hub (server and client in one application) written in Go. A chat client can subscribe to a chat room to receive chat messages and can publish messages to joined chat rooms, as well as create new chat rooms. There is also a clean web app front-end chat client for communicating without the command-line. 

### Client Console Commands
* "list" -> list all available rooms.
* "create room_name" -> Create a chat room.
* "destroy room_name" -> Destroy a chat room (creator of room only).
* "join room_name" -> Join an existing chat room.
* "leave room_name" -> Leave a chat room.
* "room_name message" -> Send a message to all users in a chat room.
* "exit" -> Exit client.

### TODO
* Persist rooms on restart
* Add hostname flag at startup
* Auto join room on join success (on UI)