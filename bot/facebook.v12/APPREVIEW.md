# **APP** Review

---

App Review is part of app development that enables us to verify that your app uses our Products and APIs in an approved manner. If your app will be used by anyone without a Role on the app or a role in a Business that has claimed the app, it must first undergo App Review. [About](https://developers.facebook.com/docs/app-review)

---

## **Facebook** Messenger

---

The Messenger Platform Overview details how the platform works and what you need to successfully implement the platform.

Messenger from Meta is a messaging service that allows a business' Facebook Page or Instagram Professional account to respond to people who are interested in your business or social media. Conversations between a person and your account must be initiated by the person.

[Requirements](https://developers.facebook.com/docs/messenger-platform/get-started#requirements)

---

 Permission or Feature | Endpoints | Description | Note
--------------------------|-------------|----------|------
**pages_show_list** | [/user/accounts]() | **Permission** allows your app to access the list of Pages a person manages. The allowed usage for this permission is to show a person the list of Pages they manage and verify that a person manages a Page. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/permissions/reference/pages_show_list) | **FIXME**: For cloud-based instalation. *Standard Access* is enough for App Admin(s) to get list of their pages.
**pages_messaging**| [/page/messages]()| **Permission** allows your app to manage and access Page conversations in Messenger. The allowed usage for this permission is to create user-initiated interactive experiences, send customer support messages or to confirm bookings or purchases and orders. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/permissions/reference/pages_messaging)
**pages_manage_metadata** | [/page/settings]()<br>[/page/subscribed_apps]() | **Permission** allows your app to subscribe and receive webhooks about activity on the Page, and to update settings on the Page. The allowed usage for this permission is to help a Page Admin administer and manage a Page. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata) | Permissions to message people who do not have a role on your app
**Business Asset User Profile Access**| [/user]() | **Feature** allows your app to read the User Fields for users engaging with your business assets such as id, ids_for_business, name, and picture. The allowed usage for this feature is to read one or more of the User Fields in a business app experience. You may also use this feature to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/features-reference/business-asset-user-profile-access)

---

## **Instagram** Messenger

---

The Messenger Platform allows you to build messaging solutions for Instagram Professional accounts at scale.

Instagram Messaging is available for the following accounts:

 * Any Instagram Professional account for a business
 * Any Instagram Professional account for a Creator

[Requirements](https://developers.facebook.com/docs/messenger-platform/instagram)

---

 Permission | Endpoints | Description | Note
--------------------------|-------------|----------|------
**instagram_basic** | [/ig-user]()<br/>[/ig-media]()<br/>[/ig-comment]() | Allows your app to read an Instagram account profile's info and media. The allowed usage for this permission is to get basic metadata of an Instagram Business account profile, for example username and ID. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/permissions/reference/instagram_basic)
**instagram_manage_messages** | [/page/messages]() | Allows business users to read and respond to Instagram Direct messages. The allowed usage for this permission is for businesses to retrieve threads and messages from its IG Direct inbox, manage messages with their customers or to use third-party customer relationship management (CRM) tools to manage its IG Direct inbox. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/permissions/reference/instagram_manage_messages)
**pages_manage_metadata** | [/page/settings]()<br>[/page/subscribed_apps]() | **Permission** allows your app to subscribe and receive webhooks about activity on the Page, and to update settings on the Page. The allowed usage for this permission is to help a Page Admin administer and manage a Page. You may also use this permission to request analytics insights to improve your app and for marketing or advertising purposes, through the use of aggregated and de-identified or anonymized information (provided such data cannot be re-identified). [About](https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata) | Permissions to message people who do not have a role on your app