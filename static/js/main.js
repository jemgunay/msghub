var hostname = location.protocol + '//' + location.host;
var msgMe, msgOther, roomBtn;

$(document).ready(function() {
    // empty panes
    // $('#messages-pane').empty();
    // $('#left-pane').empty();
    
    // fetch html for room buttons & self/other client messages
    performRequest(hostname + "/msg_me.html", "GET", function(html) {
        msgMe = html;
    });
    performRequest(hostname + "/msg_other.html", "GET", function(html) {
        msgOther = html;
    });
    performRequest(hostname + "/room_btn.html", "GET", function(html) {
        roomBtn = html;
    });

    // poll for new chat data
    setInterval(function() {
        // fetch room names & add to side bar
        performRequest(hostname + "/refresh/", "GET", function(response) {
            if (response.trim() !== "") {
                console.log(response);
            }
        });
    }, 1000);

    // fetch room names & add to side bar
    performRequest(hostname + "/request/list/ignore/", "POST", function(rooms) {
        // $('#left-pane').empty().append(rooms);
    });
});

function performRequest(URL, httpMethod, resultMethod) {
    $.ajax({
        url: URL,
        type: httpMethod,
        dataType: 'text',
        error: function(e) {
            //console.log(e);
        },
        success: function(e) {
            //console.log(e);
            resultMethod(e);
        }
    });
}