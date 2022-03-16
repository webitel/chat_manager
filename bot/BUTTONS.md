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
|      Type      | Telegram | Messenger |
|----------------|:--------:|:---------:|
| **`url`**      |    ✔️     |    ✔️     |
| **`reply`**    |    ✔️     |    ✔️     |
| **`postback`** |    ✔️     |    ✔️     |
| **`location`** |    ✔️     |    ✔️     |
| **`phone`**    |    ✔️     |    ✔️     |
| **`email`**    |    ✖     |    ✔️     |
| **`clear`**    |    ✔️     |    ✖     |
