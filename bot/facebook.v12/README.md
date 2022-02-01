# Facebook Messenger **Bot API**

> The Messenger Bot's Pages **API**s are available thru HTTP **GET `/{bot-callback-uri}`**</br>endpoint with specified **URL** *?query=* parameters as described below:

Request | Description
--------|-------------
`?pages=`**`setup`** | *Add or Remove Messenger Pages (Accounts)</br> on behalf of the Page's Administrative Role*
`?pages=[`**`search`**`[&`**`id`**`=,...&`**`id`**`=]]` | *Shows list of engaged Page(s) with it's Accounts*
`?pages=`**`subscribe`**`[&`**`id`**`=,...&`**`id`**`=]` | *Subscribes Bot's Webhook on Page's update events.</br>Activate Pages*
`?pages=`**`unsubscribe`**`[&`**`id`**`=,...&`**`id`**`=]` | *Unsubscribes Bot's Webhook from Page's update events.</br>Deactivate Pages*

> This bot provider also distributes the following settings for each chat channel to help you determine which page you are contacted on behalf:

Parameter | Description
----------|------------
**`messenger_page`** | *[**A**]pp-[**s**]coped Page unique [**ID**] as an undelaying Chat recipient*
**`messenger_name`** | *Human-readable Name of the Messenger Page to display*
