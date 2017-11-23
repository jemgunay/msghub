var hostname = location.protocol + '//' + location.host;
var msgMe, msgOther, roomBtn, clientUsername;
var roomsHistory = {};
var currentRoom = null;

$(document).ready(function() {
    // fetch html for room buttons & self/other client messages
    performRequest(hostname + "/msg_me.html", "GET", "", function(html) {
        msgMe = html;
    });
    performRequest(hostname + "/msg_other.html", "GET", "", function(html) {
        msgOther = html;
    });
    performRequest(hostname + "/room_btn.html", "GET", "", function(html) {
        roomBtn = html;
    });
    performRequest(hostname + "/fetch/name/", "GET", "", function(html) {
        clientUsername = html;
        $("#msg-input").attr("placeholder", clientUsername + ", type your message here...");
    });
    
    // wait for required html to be fetched before refreshing
    setTimeout(function() {
        mainWaitLoop();    
    }, 500);
    
    // fetch room names & add to side bar
    performRequest(hostname + "/request/", "POST", {Type: "list"}, function(rooms) {});
    
    // send message on button click
    $("#input-pane button").on("click", function(e) {
        sendMessage();
    });
    // send message on enter key press
    $(document).keypress(function(e) {
        if(e.which == 13 && $("#msg-input").is(":focus")) {
            sendMessage();
        }
    });
    
    // handle room control button actions
    $(".room-control-btn").on("click", function(e) {
        switch ($(this).attr("id")) {
            // refresh room list
            case "refresh-btn":
                performRequest(hostname + "/request/", "POST", {Type: "list"}, function(rooms) {});
                break;
            // join a room
            case "join-btn":
                var roomName = prompt("Enter the name of the room to join:", "");
                performRequest(hostname + "/request/", "POST", {Type: "join", Room: roomName}, function(rooms) {});
                break;
            // leave a room
            case "leave-btn":
                var roomName = prompt("Enter the name of the room to leave:", "");
                performRequest(hostname + "/request/", "POST", {Type: "leave", Room: roomName}, function(rooms) {});
                break;
            // create a room
            case "create-btn":
                var roomName = prompt("Enter the name of the room to create:", "");
                performRequest(hostname + "/request/", "POST", {Type: "create", Room: roomName}, function(rooms) {});
                break;
            // destroy a room
            case "destroy-btn":
                var roomName = prompt("Enter the name of the room to destroy:", "");
                performRequest(hostname + "/request/", "POST", {Type: "destroy", Room: roomName}, function(rooms) {});
                break;
            // exit client & window
            case "exit-btn":
                var confirmation = confirm("Are you sure you want to exit?");
                if (confirmation == true) {
                    performRequest(hostname + "/fetch/exit/", "GET", "", function(html) {});
                    setTimeout(function() {
                        window.close();
                    }, 500);
                    
                    break;
                }
        }
    });
});

// Continuously retrieve new chat data.
function mainWaitLoop() {
    // poll for new chat data
    setInterval(function() {
        // update message window with currently selected room's data feed
        if (currentRoom != null) {
            $("#messages-pane").empty().append(roomsHistory[currentRoom]);
        }
        
        // fetch room names & add to side bar
        performRequest(hostname + "/refresh/", "GET", "", function(response) {
            if (response.trim() !== "") {
                // parse response as json
                var jsonResponse = JSON.parse(response);

                // init room data to string if null
                if (roomsHistory[jsonResponse.Room] == null) {
                    roomsHistory[jsonResponse.Room] = "";
                }
                
                switch (jsonResponse.Type) {
                    case "list":
                        $('#chat-rooms').empty();
                        var rooms = jsonResponse.Text.split(", ");
                        rooms.sort();
                        // iterate over chat room names and append to page
                        for (var room in rooms) {
                            var roomBtnPopulated = roomBtn.replace("name_placeholder", rooms[room]);
                            $('#chat-rooms').append(roomBtnPopulated);
                            // make button clickable
                            $(".room-btn").on("click", function(e) {
                                e.preventDefault();
                                currentRoom = $(this).html();
                                $(".well").css("background-color", "#ADB6B5");
                                $(this).closest(".well").css("background-color", "#909393");
                            });
                        }

                    case "new_msg":
                    case "join":
                    case "leave":
                        logChatMessage(jsonResponse);
                        break;
                    case "create":
                    case "destroy":
                        logChatMessage(jsonResponse);
                        // fetch room names & add to side bar
                        setTimeout(function() {
                            performRequest(hostname + "/request/", "POST", {Type: "list"}, function(rooms) {});
                        }, 500);
                        
                        break;

                    default:
                        console.log("Unrecognised response type: " + jsonResponse.Type);
                }
            }
        });
    }, 100);
}

function sendMessage() {
    if (currentRoom == null) {
        alert("Please select a room to the left to send a message to.");
        return
    }
    performRequest(hostname + "/request/", "POST", {Type: "new_msg", Room: currentRoom, Text: $("#msg-input").val()}, function(rooms) {});
    $("#msg-input").val("");
}

// Add new chat message to corresponding array log.
function logChatMessage(jsonResponse) {
    var targetTemplate = msgOther;
    if (jsonResponse.Username === clientUsername) {
        targetTemplate = msgMe;
    }
    // check for error
    var message = jsonResponse.Text;
    if (jsonResponse.Error !== "") {
        message = jsonResponse.Error.charAt(0).toUpperCase() + jsonResponse.Error.slice(1);
    }
    
    // replace template variables with message data
    var msgHTMLPopulated = targetTemplate.replace("name_placeholder", jsonResponse.Username);
    msgHTMLPopulated = msgHTMLPopulated.replace("message_placeholder", message);
    roomsHistory[jsonResponse.Room] += msgHTMLPopulated;
}

// Perform AJAX request.
function performRequest(URL, httpMethod, data, resultMethod) {
    $.ajax({
        url: URL,
        type: httpMethod,
        dataType: 'text',
        data: data,
        error: function(e) {
            console.log(e);
        },
        success: function(e) {
            //console.log(e);
            resultMethod(e);
        }
    });
}