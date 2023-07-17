# **Access** Levels

[Access levels](https://developers.facebook.com/docs/graph-api/overview/access-levels/) are an additional layer of Graph API authorization that apply to permissions and features for Business and Consumer apps.

There are two access levels: **Standard** and **Advanced**. Apps can request permissions with Advanced Access from any app user, and features with Advanced Access are active for all app users. Permissions with Standard Access, however, can only be requested from app users who have a role on the requesting app, and features with Standard Access are only active for app users who have a role on the app.

If your app will only be used by people who have a role on it, the permissions and features your app requires will only need Standard Access. If your app will be used by people who do not have a role on it, the permissions and features that your app requires will need Advanced Access.

All Business and Consumer apps are automatically approved for Standard Access for all permissions and features. Advanced Access, however, must be approved on an individual permission and feature basis through the App Review process.

---

# **Application** Review

[App Review](https://developers.facebook.com/docs/app-review) is part of app development that enables **Meta** to verify that your app uses their Products and APIs in an approved manner. If app will be used by anyone without a Role on the app or a role in a Business that has claimed the app, it must first undergo App Review.

---

# **Facebook** Messenger

[Messenger](https://developers.facebook.com/docs/messenger-platform) from Meta is a messaging service that allows a business **Facebook Page** or **Instagram Professional** account to respond to people who are interested in your business or social media. Conversations between a person and your account must be initiated by the person. [Requirements](https://developers.facebook.com/docs/messenger-platform/get-started#requirements)

---

The table below describes the set of permissions that **Webitel** uses to integrate with **Facebook Messenger** Page.

 Permission or Feature | Endpoints | Description | Note
--------------------------|-------------|----------|------
[pages_show_list](https://developers.facebook.com/docs/permissions/reference/pages_show_list)|[/user/accounts](https://developers.facebook.com/docs/graph-api/reference/user/accounts)| **Permission** allows your app to access the list of Pages a person manages. The allowed usage for this permission is to show a person the list of Pages they manage and verify that a person manages a Page. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). | **FIXME**: For cloud-based instalation. *Standard Access* is enough for App Admin(s) to get list of their pages.
[pages_messaging](https://developers.facebook.com/docs/permissions/reference/pages_messaging)|[/page/messages](https://developers.facebook.com/docs/graph-api/reference/page/messages/)| **Permission** allows your app to manage and access Page conversations in Messenger. The allowed usage for this permission is to create user-initiated interactive experiences, send customer support messages or to confirm bookings or purchases and orders. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified).
[pages_manage_metadata](https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata)|[/page/settings](https://developers.facebook.com/docs/graph-api/reference/page/settings/)<br>[/page/subscribed_apps](https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/)| **Permission** allows your app to subscribe and receive webhooks about activity on the Page, and to update settings on the Page. The allowed usage for this permission is to help a Page Admin administer and manage a Page. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). | Permissions to message people who do not have a role on your app
[Business Asset User Profile Access](https://developers.facebook.com/docs/features-reference/business-asset-user-profile-access)|[/user](https://developers.facebook.com/docs/graph-api/reference/user/)| **Feature** allows your app to read the User Fields for users engaging with your business assets such as id, ids_for_business, name, and picture. The allowed usage for this feature is to read one or more of the User Fields in a business app experience. You may also use this feature to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified).

---

# **Instagram** Messenger

The [Messenger](https://developers.facebook.com/docs/messenger-platform/instagram) Platform allows you to build messaging solutions for **Instagram Professional** accounts at scale.

Instagram Messaging is available for the following accounts:

 * Any Instagram Professional account for a business
 * Any Instagram Professional account for a Creator

---

The table below describes the set of permissions that **Webitel** uses to integrate with **Instagram Messenger** Page.

 Permission | Endpoints | Description | Note
--------------------------|-------------|----------|------
[instagram_basic](https://developers.facebook.com/docs/permissions/reference/instagram_basic)|[/ig-user](https://developers.facebook.com/docs/instagram-api/reference/ig-user)<br/>[/ig-media](https://developers.facebook.com/docs/instagram-api/reference/ig-media)<br/>[/ig-comment](https://developers.facebook.com/docs/instagram-api/reference/ig-comment)| Allows your app to read an Instagram account profile's info and media. The allowed usage for this permission is to get basic metadata of an Instagram Business account profile, for example username and ID. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified).
[instagram_manage_messages](https://developers.facebook.com/docs/permissions/reference/instagram_manage_messages)|[/page/messages](https://developers.facebook.com/docs/graph-api/reference/page/messages/)| Allows business users to read and respond to Instagram Direct messages. The allowed usage for this permission is for businesses to retrieve threads and messages from its IG Direct inbox, manage messages with their customers or to use third-party customer relationship management (CRM) tools to manage its IG Direct inbox. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified).
[pages_manage_metadata](https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata)|[/page/settings](https://developers.facebook.com/docs/graph-api/reference/page/settings/)<br>[/page/subscribed_apps](https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/)| **Permission** allows your app to subscribe and receive webhooks about activity on the Page, and to update settings on the Page. The allowed usage for this permission is to help a Page Admin administer and manage a Page. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified).| Permissions to message people who do not have a role on your app

---

# **Graph API**

The table lists the [Graph API](https://developers.facebook.com/docs/graph-api)s we use to make it all work together

 Methods | Endpoint          | Permissions | About
---------|-------------------|-------------|-------
`GET`|[/user/accounts](https://developers.facebook.com/docs/graph-api/reference/v13.0/user/accounts)|Page access token<br/>**pages_show_list**|The Facebook Pages that a person owns or is able to perform tasks on.
`GET`|[/user?fields=name](https://developers.facebook.com/docs/graph-api/reference/user)|Page access token<br/>**public_profile**<br/>**Business Asset User Profile Access**|Read the User Fields for users engaging with your business assets such as id, ids_for_business, name, and picture.
`GET`|[/page?fields=instagram_business_account](https://developers.facebook.com/docs/instagram-api/reference/page)|User access token<br/>**instagram_basic**<br/>**pages_show_list**|This node allows you to get the [IG User](https://developers.facebook.com/docs/instagram-api/reference/ig-user) connected to a Facebook Page.
`POST`|[/page/messages](https://developers.facebook.com/docs/messenger-platform/reference/send-api)|Page access token<br/>**pages_messaging**<br/>**instagram_manage_messages**|The Send API used to send messages to users, including text, attachments, structured message templates, sender actions, and more.
`GET`<br/>`POST`<br/>`DELETE`|[/page/subscribed_apps](https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps)|Page access token<br/>**pages_manage_metadata**<br/>**pages_show_list**|Webhook subscriptions for apps that service a Facebook Page.
`GET`<br/>`POST`<br/>`DELETE`|[/app/subscriptions](https://developers.facebook.com/docs/graph-api/reference/v15.0/app/subscriptions)|App access token|This edge allows you to read, create, update, and delete Webhooks subscriptions.
