# Meta Messenger **Bot API**

> The Messenger Bot's Pages **API**s are available thru HTTP **GET `/{bot-callback-uri}`**</br>endpoint with specified **URL** *?query=* parameters as described below:

GET /$callbackURI | Description
------------|-------------
**`?pages=setup`** | *Add or Remove **Messenger** Pages (Accounts)</br> on behalf of the Page's Administrative Role*
**`?pages=[search[&id=]]`** | *Shows list of engaged Page(s) with it's Accounts*
**`?pages=subscribe[&id=]`** | *Subscribes Bot's Webhook on Page's update events.</br>Activate Pages*
**`?pages=unsubscribe[&id=]`** | *Unsubscribes Bot's Webhook from Page's update events.</br>Deactivate Pages*
**`?instagram=setup`** | *Add or Remove **Instagram** Pages (Accounts)</br> on behalf of the Page's Administrative Role*
**`?instagram=[search[&id=]]`** | *Shows list of engaged Page(s) with it's Accounts*

-----

> This type of chat client provider may be customized with a several parameters, type of _string_ value.
>
> <span style="color:red">**WARN**</span>: **Instagram** NOT working while your App is in *Development Mode* !!!

Metadata | Value | Usage
---------|-------|------
**`client_id`**|_REQUIRED. App-ID._|Client Authentication.
**`client_secret`**|_REQUIRED. App-Secret._|Client Authorization.
**`instagram.comments`**|_OPTIONAL. Default_: `"false"`.|Forward comment(s) on your Instagram media posts in chat ?<br/>`text: #comment`<br/>`[variables]:`<br/>`instagram.comment: $comment.text`<br/>`instagram.comment.link: $comment.link`
**`instagram.mentions`**|_OPTIONAL. Default_: `"false"`.|Forward @mention(s) of you in Instagram media posts or comments in chat ?<br/>`text: #mention`<br/>`[variables]:`<br/>`instagram.mention: $mention.text`<br/>`instagram.mention.link: $mention.link`

-----

> This type of chat client provider also distributes the following variables for each chat channel to help you determine which page you are contacted on behalf:

Parameter | Description
----------|------------
**`facebook.page`** | *[**A**]pp-[**s**]coped Facebook Account Page unique [**ID**]entifier*
**`facebook.name`** | *Facebook Account Page Name*
**`instagram.page`** | *[**I**]nsta[**G**]ram-[**s**]coped Account Page unique [**ID**]entifier*
**`instagram.user`** | *Username of the Instagram Professional or Business Account*
