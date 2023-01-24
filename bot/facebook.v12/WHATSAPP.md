# Meta **WhatsApp** Business **Bot API**

## Platform

The WhatsApp Business Platform gives medium to large businesses the ability to connect with customers at scale. You can start conversations with customers in minutes, send customer care notifications or purchase updates, offer your customers a level of personalized service and provide support in the channel that your customers prefer to be reached on.

## Cloud API

Send and receive messages using a cloud-hosted version of the WhatsApp Business Platform. The Cloud API allows you to implement WhatsApp Business APIs without the cost of hosting of your own servers and also allows you to more easily scale your business messaging.

## Bot API

The WhatsApp Bot's Accounts are available via HTTP **GET `/{bot-callback-uri}`**</br>endpoint with specified **URL** *?query=* parameters as described below:

GET /$callbackURI | Description
------------|-------------
**`?whatsapp=setup`** | *Add or Refresh **WhatsApp** Business Account(s)</br> on behalf of the Meta Business Administartive Role*
**`?whatsapp=[search][&id=]`** | ***Lookup** for engaged WhatsApp Business Account(s)*
**`?whatsapp=subscribe[&id=]`** | ***Activate** desired WhatsApp Business Account(s)*
**`?whatsapp=unsubscribe[&id=]`** | ***Deactivate** desired WhatsApp Business Account(s)*
**`?whatsapp=remove[&id=]`** | *Deactivate and **Remove** engaged WhatsApp Business Account(s)*

-----

## Gateway Configuration

This type of chat client may be customized with a several parameters, type of _string_ value.

Metadata | Value | Usage
---------|-------|------
**`whatsapp_token`**|REQUIRED. string.|*Client **Authorization** for WhatsApp Business Platform Cloud-API integration*

-----

## Dialog Environment

This type of chat client also propagates the following variables for each chat channel to help you identify which of your WhatsApp Business Account the customer is contacted:

Variable | Description
---------|------------
**`whatsapp.business`** | *[**W**]hats[**A**]pp[**B**]usiness[**A**]ccount unique [**ID**]entifier*
**`whatsapp.account`** | *[**W**]hats[**A**]pp account's [**Number**] unique [**ID**]entifier*
**`whatsapp.number`** | *[**W**]hats[**A**]pp account's [**Phone Number**] engaged*

---

# [**Get Started With the WhatsApp Business Cloud API**](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started)

## 1. [Set up Developer Assets and Platform Access](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started#set-up-developer-assets)

The [WhatsApp Cloud API](https://developers.facebook.com/docs/whatsapp/cloud-api) are part of Meta’s Graph API, so you need to set up a Meta developer account and a Meta developer app. To set that up:

* [Register as a Meta Developer](https://developers.facebook.com/docs/development/register)
* [Enable two-factor authentication for your account](https://www.facebook.com/help/148233965247823)
* [Create a Meta App](https://developers.facebook.com/docs/development/create-an-app/): Go to **developers.facebook.com** > **My Apps** > **Create App**.</br>Select the "Business" type and follow the prompts on your screen.

From the App Dashboard, click on the app you would like to connect to WhatsApp. Scroll down to find the "WhatsApp" product and click **Set up**.

Next, you will see the option to select an existing Business Manager (if you have one) or, if you would like, the onboarding process can create one automatically for you (you can customize your business later, if needed).

Make a selection and click Continue. This will:

1. Associate your app with the Business Manager account that you selected earlier (or had created for you).
2. Generate a WhatsApp Business account.
3. Generate a test business phone number and associate it with your WhatsApp Business Account. You can use this number with the API to send an unlimited number of messages to up to 5 recipient phone numbers. Recipient phone numbers can be any valid number, but you must verify each one in the next step.
4. Redirect you to the **WhatsApp** > **Getting Started** (or **Setup**) panel in the App Dashboard.

## 2. [Generate Meta Business Manager System User permanent access token](https://digitalinspiration.com/docs/document-studio/apps/whatsapp/token)

To create a permanent access token to make calls to the WhatsApp Business Platform Cloud API as an authenticated user.

1. Inside your Facebook App Dashboard, Go to **Business Settings** > **Users** > **System Users**. Click on the **Add** button to create a new system user.
Give your system user a unique username and set the role to **Admin**. Click on **Create System user** to continue. You'll be redirected to the system user's profile page.

2. Click on **Add Assets** to assign your app to the system user. Go to the **Apps** tab and select your app from the list. Turn on the **Full Control** > **Manage App** option and click on **Save Changes**.

3. Click on **Generate New Token** to generate a permanent access token for your system user. Select your app name from the dropdown and enable the following permissions.

   * **whatsapp_business_management**
   * **whatsapp_business_messaging**

Click on **Generate Token**, and an access token will be generated for your system user. Please copy the access token in your notepad as it will not be visible again in your Facebook dashboard.

Unlike the temporary access token, this permanent access token will never expire unless you manually revoke it.

Your app is now ready to send WhatsApp messages. You can use this permanent access token to integrate your WhatsApp Business Account(s) with **Webitel** > **Chat Gateway** > type:**Messenger** > tab:**WhatsApp**.

> **NOTE**: _For more information see how to [Add System Users to Your Meta Business Manager](https://www.facebook.com/business/help/503306463479099), [Install Apps and Generate Tokens](https://developers.facebook.com/docs/marketing-api/system-users/install-apps-and-generate-tokens#install-apps-and-generate-tokens)._

## [Phone Number](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started#phone-number)

For a production use case, you need to use your own phone number to send messages to your users. When choosing a phone number, consider the following:

* If you want to use a number that is already being used in the WhatsApp customer or business app, you will have to fully migrate that number to the business platform. Once the number is migrated, you will lose access to the WhatsApp customer or business app. See [Migrate Existing WhatsApp Number to a Business Account](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started/migrate-existing-whatsapp-number-to-a-business-account) for information.
* Meta WhatsApp Business Platform have a set of rules regarding numbers that can be used. [Learn more](https://developers.facebook.com/docs/whatsapp/phone-numbers#pick-number).
* Once you have chosen your phone number, you have to add it to your WhatsApp Business Account. See [Add a Phone Number](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started/add-a-phone-number).

---

## 3. [Add a Phone Number](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started/add-a-phone-number)

Navigate to the Meta App that is set-up for WhatsApp by going to **developers.facebook.com** > **My Apps** > **Select your App**.

> **NOTE**: _If your phone number is currently registered with WhatsApp Messenger or the WhatsApp Business App, you need to first delete it. See [Migrate an Existing WhatsApp Number to a Business Account](https://developers.facebook.com/docs/whatsapp/cloud-api/get-started/migrate-existing-whatsapp-number-to-a-business-account)._

1. Use left menu to navigate to the product **WhatsApp** > **Getting Started** (or **Setup**) panel.
2. On the right pane select Add phone number button under **Step 5: Add a phone number**.
3. Use the [display name guidelines](https://www.facebook.com/business/help/338047025165344#display-name-guidelines) to enter a display name for your phone number. This is the name that will show for your business phone number once approved.
4. Select your **Timezone**. This will be used for WhatsApp Billing and analytics.
5. Select a **Category** for your business and enter a **Business description**.
6. Select **Next** to begin the phone number verification process.
7. Select your country code from the drop down and enter the phone number you would like to register.
8. Select how you would like to receive your verification code, either by **Text Message** or **Phone** and click **Next** to continue. _You will need access to the phone number before selecting Next to receive the verification code_.
9. Enter the verification code once received and click **Next** to continue.
10. The phone number will appear in the **From** drop down menu of the **Send and receive messages** section of the panel.
11. Select the newly added phone number to begin sending messages.

> **NOTE**: _If your phone number fails to register, you will see a message below the drop down that says "Please register your phone number to send messages." Click on the link to access the configuration screen, then click on the slider button next to **Registered** to register your phone number again._