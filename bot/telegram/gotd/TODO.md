# TODO

```
[+] Помилки рівнів: code, 2fa
[+] Центрувати вводи коду, пароля
[+] Показати/приховати пароль
[+] При введені коду - бачу для якого номеру; При введені паролю - ні !
[+] Картка провайдера: API-ID, API-Hash перенести все на праву сторону.

[ ] Запускати лише з активною ліцензією домену !!!
[ ] Додати .logOut перед видаленням
[+] Після розсилки на невідомого контакта - не зберігає кеш ?! peer.(*User).AccessHash == 0 !
[+] Need to run such clients on startup to initiate state connection
[ ] Add singleton; leader attendant
[+] Save login.logout_tokens dataset
[+] Deal with peer.Resolver.UserID()
[?] HTTP API Authorization ! NO Public HTTP, gRPC only !
[!] AccessHasher persistent storage; Fill on startup: messages.getDialogs
[+] Imit real user actions; readHistoryInbox
[+] Handle remote logout (session kill)
[+] Broadcast message; What we accept as an inputPeer to broadcast to ? phoneNumber !
[?] Cleanup internal storage onLoggedOut
[+] Зберігати сесію лише після успішної авторизації !!!

```

```
← updates.getDifference
  pts: 81
  date: 2022-08-05T13:41:09Z
  qts: 0
→ (188ms) updates.difference
  new_messages: 

  new_encrypted_messages: 

  other_updates: 
  - updateReadHistoryInbox
      peer: peerUser
        user_id: 25373318
      max_id: 43
      still_unread_count: 0
      pts: 83
      pts_count: 0
  - updateReadHistoryOutbox
      peer: peerUser
        user_id: 616650894
      max_id: 41
      pts: 83
      pts_count: 0

  chats: 

  users: 
  - user
      apply_min_photo: true
      id: 25373318
      access_hash: 1820450244150522128
      first_name: Rita
      username: kadatckin
      status: userStatusRecently
  - user
      apply_min_photo: true
      id: 616650894
      access_hash: 3914219160660387614
      first_name: Олександра
      username: aleksa_andre
      photo: userProfilePhoto
        photo_id: 2648495423235401762
        stripped_thumb: AQgIiZmd_vBccgCiiigD
        dc_id: 2
      status: userStatusRecently

  state: updates.state
    pts: 83
    qts: 0
    date: 2022-08-05T13:41:11Z
    seq: 7
    unread_count: 0






2022-08-05 16:58:39.527779  file=wrapper/wrapper.go:154 level=debug Serving request for service webitel.chat.bot endpoint Messages.BroadcastMessage
← contacts.resolvePhone
  phone: 380730367300
→ (146ms) contacts.resolvedPeer
  peer: peerUser
    user_id: 231533278
  chats: 

  users: 
  - user
      apply_min_photo: true
      id: 231533278
      access_hash: -3280891081905723607
      first_name: Igor
      last_name: Igor
      username: Navrotskyj
      status: userStatusRecently

← messages.sendMessage
  peer: inputPeerUser
    user_id: 231533278
    access_hash: -3280891081905723607
  message: Привіт, SІгор 😈
 Ми знайомі?
  random_id: 8958749902363063950
→ (179ms) updateShortSentMessage
  out: true
  id: 65
  pts: 125
  pts_count: 1
  date: 2022-08-05T13:58:39Z
4:58PM DBG <<<<< SERVE <<<<< endpoint=Messages.BroadcastMessage from-service=workflow remote=10.9.8.111:49112 req={"from":272,"message":{"text":"Привіт, SІгор 😈\n Ми знайомі?","type":"text"},"peer":["+380730367300"]} spent=325.630724ms user-agent=grpc-go/1.40.0
4:58PM DBG updateNewMessage bot=631 channel=gotd entities={"Channels":{},"Chats":{},"Short":false,"Users":{}} pdc=1 pid=272 title="Telegram +380993613314" update={"Message":{"Flags":0,"ID":65,"PeerID":null},"Pts":125,"PtsCount":1} uri=/tuujpochcdcbsbfzghomtpcurkmbogp
u: updates
  updates: 
  - updateReadHistoryOutbox
      peer: peerUser
        user_id: 231533278
      max_id: 65
      pts: 126
      pts_count: 1

  users: 

  chats: 

  date: 2022-08-05T13:58:40Z
  seq: 0





































u: updateShort
  update: updateUserTyping
    user_id: 231533278
    action: sendMessageTypingAction
  date: 2022-08-05T13:58:43Z
u: updateShort
  update: updateUserStatus
    user_id: 5418416457
    status: userStatusOffline
      was_online: 1659707924
  date: 2022-08-05T13:58:43Z
u: updateShort
  update: updateNewMessage
    message: message
      id: 66
      from_id: peerUser
        user_id: 231533278
      peer_id: peerUser
        user_id: 231533278
      date: 2022-08-05T13:58:44Z
      message: +
    pts: 127
    pts_count: 1
  date: 2022-08-05T13:58:44Z
4:58PM DBG updateNewMessage bot=631 channel=gotd entities={"Channels":{},"Chats":{},"Short":false,"Users":{}} pdc=1 pid=272 title="Telegram +380993613314" update={"Message":{"Date":1659707924,"EditDate":0,"EditHide":false,"Entities":null,"Flags":256,"Forwards":0,"FromID":{"UserID":231533278},"FromScheduled":false,"FwdFrom":{"ChannelPost":0,"Date":0,"Flags":0,"FromID":null,"FromName":"","Imported":false,"PostAuthor":"","PsaType":"","SavedFromMsgID":0,"SavedFromPeer":null},"GroupedID":0,"ID":66,"Legacy":false,"Media":null,"MediaUnread":false,"Mentioned":false,"Message":"+","Noforwards":false,"Out":false,"PeerID":{"UserID":231533278},"Pinned":false,"Post":false,"PostAuthor":"","Reactions":{"CanSeeList":false,"Flags":0,"Min":false,"RecentReactions":null,"Results":null},"Replies":{"ChannelID":0,"Comments":false,"Flags":0,"MaxID":0,"ReadMaxID":0,"RecentRepliers":null,"Replies":0,"RepliesPts":0},"ReplyMarkup":null,"ReplyTo":{"Flags":0,"ReplyToMsgID":0,"ReplyToPeerID":null,"ReplyToScheduled":false,"ReplyToTopID":0},"RestrictionReason":null,"Silent":false,"TTLPeriod":0,"ViaBotID":0,"Views":0},"Pts":127,"PtsCount":1} uri=/tuujpochcdcbsbfzghomtpcurkmbogp
← users.getUsers
  id: 
  - inputUser
      user_id: 231533278
      access_hash: 0

→ (111ms) 
  Elems: 

4:58PM ERR telegram/updateNewMessage.peer error="got empty result for InputUser{UserID:231533278 AccessHash:0}" bot=631 channel=gotd pdc=1 peer={"UserID":231533278} pid=272 title="Telegram +380993613314" uri=/tuujpochcdcbsbfzghomtpcurkmbogp




```