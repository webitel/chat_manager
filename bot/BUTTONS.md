# **BUTTONS**

## **Model**

---
|   Field    |     Type     |  About |
|------------|:------------:|--------|
| **`type`** | string, enum | *Type of the button.</br> All available types are listed below.* |
| **`text`** | string       | *Label text on the button.* |
| **`code`** | string       | *Data to be sent in a callback query</br> when button is pressed, 1-64 bytes.* |
| **`url`**  | string       | *HTTP URL to be opened when the button is pressed.* |

## **Types**

---
|      Type      |     Fields    |  About |
|----------------|:-------------:|--------|
| **`url`**      | text, url     | *HTTP URL as an external link to be opened when the button is pressed.* |
| **`reply`**    | text, code    | *Quick Reply button allow you to get message recipient input by sending buttons in a message.* |
| **`postback`** | text, code    | *The postback button allows you to have an always-on user interface element inside the conversation. This is an easy way to help people discover and access the core functionality of your bot at any point in the conversation.* |
| **`location`** | text          | *The user's current location will be sent when the button is pressed.* |
| **`phone`**    | text          | *The user's phone number will be sent as a contact when the button is pressed.* |
| **`email`**    | text          | *The user's email address will be sent as a contact when the button is pressed.* |
| **`clear`**    |               | *Removes the current custom postback buttons keyboard* |

## **Support**

---

|                | Viber | Telegram | Facebook  | Instagram | WhatsApp  |
|----------------|:-----:|:--------:|:---------:|:---------:|:---------:|
| **`url`**      |   ✔️   |    ✔️     |     ✔️     |     ✔️     |     ✖     |
| **`reply`**    |   ✔️   |    ✔️     |     ✔️     |     ✔️     |     ✔️     |
| **`postback`** |   ✔️   |    ✔️     |     ✔️     |     ✔️     |     ✔️     |
| **`location`** |   ✔️   |    ✔️     |     ✖     |     ✖     |     ✖     |
| **`phone`**    |   ✔️   |    ✔️     |     ✔️     |     ✖     |     ✖     |
| **`email`**    |   ✖   |    ✖     |     ✔️     |     ✖     |     ✖     |
| **`clear`**    |   ✖   |    ✔️     |     ✖     |     ✖     |     ✖     |

---

# **BUTTON** Requirements.

|   Provider(s)   | Text (Display) | Code (Postback) | Link(s)
|-----------------|-------------|-------------|--------------
| **`Viber`**     | The keyboard is divided into 6 columns. Each button has a width of 1-6 columns. The client will display the buttons according to the order they were sent in, and will fit as many buttons as possible into each row. Keyboards can contain up to 24 rows.</br></br>Max 250 characters. If the text is too long to display on the button it will be cropped and ended with “…” |  | https://developers.viber.com/docs/tools/keyboards/#keyboard-design
| **`Telegram`**  | Label text on the button | Data to be sent in a callback query to the bot when button is pressed, 1-64 bytes | https://core.telegram.org/bots/api#inlinekeyboardbutton</br>https://core.telegram.org/bots/api#keyboardbutton
| **`Facebook`**  | Button title. 20 character limit.</br>`.quick_replies[1~13]`</br>A maximum of 13 quick replies are supported.</br>`.attachment{type:button}.payload.buttons[1~3]`</br>Set of 1-3 buttons that appear as call-to-actions.</br>`.attachment{type:generic}.payload.elements[1].buttons[1~3]`</br>elements: A maximum of 1 element is supported.</br>buttons: A maximum of 3 buttons per element is supported. | 1000 character limit. | https://developers.facebook.com/docs/messenger-platform/send-messages/buttons</br>https://developers.facebook.com/docs/messenger-platform/reference/buttons
| **`Instagram`** | A maximum of **13 quick replies** are supported and each quick reply allows up to 20 characters before being truncated. Quick replies only support plain text. When a quick reply is tapped, the buttons are dismissed. |             | https://developers.facebook.com/docs/messenger-platform/instagram/features/quick-replies</br>https://developers.facebook.com/docs/messenger-platform/instagram/features/generic-template
| **`WhatsApp`**  | `.interactive{type:button}.action.buttons[1~3]`</br>You can have **up to 3 buttons**!</br>It cannot be an empty string and must be unique within the message. Emojis are supported, markdown is not. Maximum length: 20 characters.</br>`.interactive{type:list}.action.sections[1].rows[1~10]`</br>You can have a **total of 10 rows across your sections**!</br>Each row must have a title (Maximum length: 24 characters). | `.interactive{type:button}.action.buttons[1~3]`</br>Maximum length: 256 characters.</br>`.interactive{type:list}.action.sections[1].rows[1~10]`</br>Maximum length: 200 characters | https://developers.facebook.com/docs/whatsapp/cloud-api/guides/send-messages#interactive-messages</br>https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#action-object</br>https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#section-object
