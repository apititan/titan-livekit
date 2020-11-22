package org.kurento.tutorial.groupcall;

import java.io.IOException;
import org.kurento.client.IceCandidate;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonObject;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class CallHandler  {

  private static final Logger log = LoggerFactory.getLogger(CallHandler.class);

  private static final Gson gson = new GsonBuilder().create();

  @Autowired
  private RoomManager roomManager;

  @PostMapping("/joinRoom")
  public void joinRoom(@RequestParam String userSessionId, @RequestParam Long roomId) throws IOException {
    joinRoom(roomId, userSessionId);
  }

  @PostMapping("/receiveVideoFrom")
  public void receiveVideoFrom(@RequestParam String userSessionId, @RequestParam Long roomId, @RequestBody JsonObject jsonMessage) throws IOException {
    final Room room = roomManager.getRoom(roomId);
    final UserSession user = room.getUserSession(userSessionId);
    if (user == null) {
      log.warn("UserSession userSessionId={} not found in room {}", userSessionId, roomId);
      return;
    }

    final String senderSessionId = jsonMessage.get("sender").getAsString();
    final UserSession sender = room.getUserSession(senderSessionId);
    if (sender == null) {
      log.warn("UserSession userSessionId={} not found in room {}", userSessionId, roomId);
      return;
    }

    final String sdpOffer = jsonMessage.get("sdpOffer").getAsString();
    user.receiveVideoFrom(sender, sdpOffer);
  }

  @PostMapping("/leaveRoom")
  public void leaveRoom(@RequestParam String userSessionId, @RequestParam Long roomId) throws IOException {
    leaveRoom(roomId, userSessionId);
  }

  @PostMapping("/onIceCandidate")
  public void onIceCandidate(@RequestParam String userSessionId, @RequestParam Long roomId, @RequestBody JsonObject jsonMessage) {
    final Room room = roomManager.getRoom(roomId);
    final UserSession user = room.getUserSession(userSessionId);
    if (user == null) {
      log.warn("UserSession userSessionId={} not found in room {}", userSessionId, roomId);
      return;
    }

    JsonObject candidate = jsonMessage.get("candidate").getAsJsonObject();

    if (user != null) {
      IceCandidate cand = new IceCandidate(
              candidate.get("candidate").getAsString(),
              candidate.get("sdpMid").getAsString(),
              candidate.get("sdpMLineIndex").getAsInt()
      );
      user.addCandidate(cand, jsonMessage.get("name").getAsString());
    }

  }
  /*
  @Override
  public void handleTextMessage(WebSocketSession session, TextMessage message) throws Exception {
    final JsonObject jsonMessage = gson.fromJson(message.getPayload(), JsonObject.class);

    final UserSession user = registry.getBySession(session);

    if (user != null) {
      log.debug("Incoming message from user '{}': {}", user.getName(), jsonMessage);
    } else {
      log.debug("Incoming message from new user: {}", jsonMessage);
    }

    switch (jsonMessage.get("id").getAsString()) {
      case "joinRoom":
        joinRoom(jsonMessage, session);
        break;
      case "receiveVideoFrom":
        final String senderName = jsonMessage.get("sender").getAsString();
        final UserSession sender = registry.getByName(senderName);
        final String sdpOffer = jsonMessage.get("sdpOffer").getAsString();
        user.receiveVideoFrom(sender, sdpOffer);
        break;
      case "leaveRoom":
        leaveRoom(user);
        break;
      case "onIceCandidate":
        JsonObject candidate = jsonMessage.get("candidate").getAsJsonObject();

        if (user != null) {
          IceCandidate cand = new IceCandidate(candidate.get("candidate").getAsString(),
              candidate.get("sdpMid").getAsString(), candidate.get("sdpMLineIndex").getAsInt());
          user.addCandidate(cand, jsonMessage.get("name").getAsString());
        }
        break;
      default:
        break;
    }
  }

  @Override
  public void afterConnectionClosed(WebSocketSession session, CloseStatus status) throws Exception {
    UserSession user = registry.removeBySession(session);
    roomManager.getRoom(user.getRoomName()).leave(user);
  }
*/
  private void joinRoom(Long roomId, String userSessionId) throws IOException {
    log.info("PARTICIPANT {}: trying to join room {}", userSessionId, roomId);

    Room room = roomManager.getRoom(roomId);
    final UserSession user = room.join(userSessionId);
  }

  private void leaveRoom(Long roomId, String userSessionId) throws IOException {
    final Room room = roomManager.getRoom(roomId);
    final UserSession userSession = room.getUserSession(userSessionId);
    if (userSession == null) {
      log.info("UserSession userSessionId={} not found in room {}", userSessionId, roomId);
    } else {
      room.leave(userSession);
    }
    if (room.getParticipants().isEmpty()) {
      roomManager.removeRoom(room);
    }
  }
}
