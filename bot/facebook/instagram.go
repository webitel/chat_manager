package facebook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	log2 "github.com/webitel/chat_manager/log"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	graph "github.com/webitel/chat_manager/bot/facebook/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook/messenger"
	"golang.org/x/oauth2"
)

// reports whether instagram_manage_comments permission
// for this Meta App supposed to be requested to use
func (c *Client) instagramManageComments() bool {
	return c != nil && (c.hookIGMediaComment != nil || c.hookIGMediaMention != nil)
}

// returns OAuth 2.0 scope used to Authorize Instagram Page(s)
func (c *Client) instagramOAuth2Scope() (scope []string) {
	// STATIC
	// https://developers.facebook.com/docs/messenger-platform/instagram/get-started#2--implement-facebook-login
	scope = []string{
		// "public_profile",
		// https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata
		"pages_manage_metadata", // GET|POST|DELETE /{page}/subscribed_apps
		// https://developers.facebook.com/docs/permissions/reference/pages_messaging
		"instagram_basic", // POST /{page}/messages (SendAPI)
		// https://developers.facebook.com/docs/permissions/reference/instagram_manage_messages
		"instagram_manage_messages",
	}
	// DYNAMIC
	if c.instagramManageComments() {
		scope = append(scope,
			// The allowed usage for instagram_manage_comments permission is
			// to read, update and delete comments of Instagram Business Accounts,
			// or to read media objects, such as Stories, of Instagram Business Accounts.
			"instagram_manage_comments",
		)
	}
	return // scope
}

// https://developers.facebook.com/docs/whatsapp/embedded-signup/manage-accounts#get-shared-waba-id-with-access-token
func (c *Client) getSharedInstagramPages(userToken *oauth2.Token) ([]*Page, error) {

	token, err := c.inspectToken(userToken)
	if err != nil {
		return nil, err
	}
	// Facebook Page IDs linked WITH Instagram account
	var PSID []string
	for _, scope := range token.GranularScopes {
		// Permission Dependencies
		// instagram_basic | pages_read_engagement
		//                 | pages_show_list
		// The `instagram_manage_messages` permission
		// allows business users to read and respond
		// to Instagram Direct messages.
		//
		// The `pages_show_list` permission
		// allows your app to access the list
		// of Pages a person manages.
		if scope.Permission == "pages_show_list" {
			PSID = append(PSID, scope.TargetIDs...) // copy
			break
		}
	}
	return c.fetchInstagramPages(
		context.TODO(), userToken.AccessToken, PSID,
	)
}

func (c *Client) fetchInstagramPages(ctx context.Context, accessUserToken string, PSID []string) ([]*Page, error) {

	n := len(PSID)
	if n == 0 {
		return nil, nil
	}

	// return IGID, nil
	form := url.Values{
		// Facebook Page(s)ID linked WITH Instagram
		"ids": []string{strings.Join(PSID, ",")},
		// https://developers.facebook.com/docs/graph-api/reference/page/#fields
		"fields": []string{strings.Join([]string{
			"id", // default
			"name",
			"access_token",
			"instagram_business_account.as(instagram){name,username}",
			// "instagram_business_account.as(instagram){name,username,profile_picture_url,followers_count,website}",
		}, ",")},
	}
	form = c.requestForm(form, accessUserToken)
	// Hide ?access_token= from query ...
	delete(form, graph.ParamAccessToken)
	// Add Authorization header BELOW ...

	// https://developers.facebook.com/docs/graph-api/reference/whats-app-business-account/phone_numbers/#Reading
	req, err := http.NewRequestWithContext(ctx,
		http.MethodGet, "https://graph.facebook.com"+
			path.Join("/", c.Version, "/")+
			"?"+form.Encode(),
		http.NoBody,
	)

	if err != nil {
		return nil, err
	}
	// Authorize GraphAPI Request
	req.Header.Add("Authorization", "Bearer "+accessUserToken)

	rsp, err := c.Client.Do(req)

	if err != nil {
		return nil, err
	}

	defer rsp.Body.Close()

	var (
		res = struct {
			// Public JSON result
			Error *graph.Error `json:"error,omitempty"`
			// Private JSON result
			data map[string]*Page
			raw  json.RawMessage
		}{
			data: make(map[string]*Page, n),
			// raw:  make(json.RawMessage, 0, res.ContentLength), // NO Content-Length Header provided !  =(
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res.raw)
	if err != nil {
		// ERR: Invalid JSON
		return nil, err
	}
	// CHECK: for RPC `error` first
	err = json.Unmarshal(res.raw, &res) // {"error"}
	if err == nil && res.Error != nil {
		// RPC: Result Error
		err = res.Error
	}
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(res.raw, &res.data)
	if err != nil {
		// ERR: Unexpected JSON result
		return nil, err
	}

	list := make([]*Page, 0, len(res.data))
	for _, node := range res.data {
		if node.IGSID() == "" {
			// NO Instagram linked ! Ignore page !
			continue
		}
		list = append(list, node)
	}
	return list, nil
}

// Retrive Facebook User profile and it's accounts (Pages) access granted
// Refresh Pages webhook subscription state
func (c *Client) getInstagramPages(token *oauth2.Token) (*UserAccounts, error) {

	// GET /me?fields=name,accounts{name,access_token}

	form := c.requestForm(url.Values{
		"fields": {"name,accounts{name,access_token,instagram_business_account.as(instagram){username}}"}, // ,profile_picture_url}}"},
	}, token.AccessToken,
	)

	req, err := http.NewRequest(http.MethodGet,
		"https://graph.facebook.com"+path.Join("/", c.Version, "me")+
			"?"+form.Encode(), nil,
	)

	if err != nil {
		return nil, err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var (
		pages []*Page
		resMe = struct {
			graph.User
			Accounts graph.Result `json:"accounts"`
		}{
			Accounts: graph.Result{
				Data: &pages,
			},
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&resMe)

	if err != nil {
		// Failed to decode JSON result
		return nil, err
	}

	if resMe.Accounts.Error != nil {
		// GraphAPI request error
		return nil, resMe.Accounts.Error
	}
	// Inspect debug_token.granular_scope granted
	pages, err = c.getSharedInstagramPages(token)
	if err != nil {
		return nil, err
	}
	// Remove page(s) that does not have .Instagram account assigned !
	for i := 0; i < len(pages); i++ {
		if pages[i].IGSID() == "" {
			if i+1 < len(pages) {
				pages = append(pages[0:i], pages[i+1:]...)
			} else {
				pages = pages[0:i]
			}
			i--
			continue
		}
	}

	res := &UserAccounts{
		User:  &resMe.User,
		Pages: pages,
		// Pages: make(map[string]*messengerPage, len(pages)),
	}

	// GET Each Page's subscription state !
	err = c.getSubscribedFields(token, pages)
	if err == nil {
		// Subscribe undelaying Facebook Page on ANY field(s)
		// to be able to receive Instagram messages update(s)...
		err = c.subscribeInstagramPages(pages)
	}

	if err != nil {
		// Failed to GET or POST Page(s) subscribed_fields (subscription) state !
		return nil, err
	}

	return res, nil
}

func (c *Client) SetupInstagramPages(rsp http.ResponseWriter, req *http.Request) {

	// USER_ACCESS_TOKEN
	token, err := c.completeOAuth(req, c.instagramOAuth2Scope()...)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadRequest)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	accounts, err := c.getInstagramPages(token)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadGateway)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	c.addInstagramPages(accounts)

	// Save Bot's NEW internal state
	var (
		dataset string
		agent   = c.Gateway
	)

	if data := c.instagram.backup(); len(data) != 0 {
		encoding := base64.RawURLEncoding
		dataset = encoding.EncodeToString(data)
	}
	// OVERRIDE OR DELETE
	err = agent.SetMetadata(
		req.Context(), map[string]string{
			// "instagram": dataset,
			"ig": dataset,
		},
	)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusInternalServerError)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	// 200 OK
	// NOTE: Static HTML to help UI close popup window !
	_ = writeCompleteOAuthHTML(rsp, nil)
}

func (c *Client) addInstagramPages(accounts *UserAccounts) {
	_ = c.instagram.setPages(accounts)
}

func (c *Client) subscribeInstagramPages(pages []*Page) error {
	// DO NOT process subscribed page(s)
	var (
		page *Page
		todo []*Page
	)
	for i := 0; i < len(pages); i++ {
		if page = pages[i]; len(page.SubscribedFields) != 0 {
			// DO NOT subscribe due to:
			//
			// Set Up Webhooks for Instagram
			// Step 2: Enable Page Subscriptions
			// Your app must enable Page subscriptions on the Page connected to the app user's account
			// by sending a POST request to the Page Subscribed Apps edge and subscribing to any Page field.
			//
			// https://developers.facebook.com/docs/graph-api/webhooks/getting-started/webhooks-for-instagram#step-2--enable-page-subscriptions
			if todo == nil {
				todo = make([]*Page, 0, len(pages)-1)
				todo = append(todo, pages[0:i]...)
			}
			continue // OMIT
		}
		if todo != nil {
			todo = append(todo, page)
		}
	}

	if todo != nil {
		pages = todo
	}

	if len(pages) == 0 {
		// NO Pages to Subscribe ! -OR-
		// ALLready Subscribed at least on ANYone field(s)
		return nil
	}

	// NOTE: Your app must enable Page subscriptions on the Page connected to the app user's account
	// by sending a POST request to the Page Subscribed Apps edge and subscribing to ANY Page field.
	// https://developers.facebook.com/docs/graph-api/webhooks/getting-started/webhooks-for-instagram#step-2--enable-page-subscriptions
	//
	// (#100) The parameter subscribed_fields is required.
	fields := instagramPageFields

	// Do subscribe for page(s) webhook updates
	err := c.subscribePages(pages, fields)

	if err != nil {
		return err
	}

	return nil
}

func (c *Client) SubscribeInstagramPages(pageIds ...string) ([]*Page, error) {

	// Find ALL requested page(s)...
	pages, err := c.instagram.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// Do subscribe for page(s) webhook updates
	err = c.subscribeInstagramPages(pages)

	if err != nil {
		return nil, err
	}

	return pages, nil
}

func (c *Client) UnsubscribeInstagramPages(pageIds ...string) ([]*Page, error) {

	// Find ALL requested page(s)...
	pages, err := c.instagram.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// Do subscribe for page(s) webhook updates
	err = c.unsubscribePages(pages)

	if err != nil {
		return nil, err
	}

	return pages, nil
}

func (c *Client) GetInstagramPages(rsp http.ResponseWriter, req *http.Request) {

	// TODO: Authorization Required

	query := req.URL.Query()
	pageId := Fields(query["id"]...)

	pages, err := c.instagram.getPages(pageId...)

	if err != nil {
		http.Error(rsp, err.Error(), http.StatusNotFound)
		return
	}

	sort.SliceStable(pages, func(i, j int) bool { return pages[i].ID < pages[j].ID })

	header := rsp.Header()
	header.Set("Pragma", "no-cache")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "close")
	header.Set("Content-Type", "application/json; charset=utf-8") // res.Header.Get("Content-Type"))

	indent := "  "
	enc := json.NewEncoder(rsp)
	enc.SetIndent(indent, indent)

	// _ = enc.Encode(pages)

	// JSON StartArray
	_, _ = rsp.Write([]byte("["))

	// Result View
	var (
		n    int
		item = Page{
			Page: &graph.Page{
				// Envelope: Sanitized View
			},
		}
	)
	// Sanitize fields
	for _, page := range pages {

		if len(page.Accounts) == 0 {
			continue // DO NOT Show !
		}

		// JSON ArrayItem
		if n == 0 {
			indent = "\n" + indent
		} else if n == 1 {
			indent = ", "
		}
		_, _ = rsp.Write([]byte(indent))

		item.Page.ID = page.ID
		item.Page.Name = page.Name
		item.Page.Instagram = page.Instagram
		// item.Page.Picture     = page.Picture
		// item.Page.AccessToken = page.GetAccessToken()

		item.Accounts = page.Accounts
		// item.SubscribedFields = page.SubscribedFields
		// // ["name", "messages", "messaging_postbacks"]

		_ = enc.Encode(item)
		n++ // Output: Count
	}
	// JSON EndArray
	_, _ = rsp.Write([]byte("]"))
}

type Timestamp time.Time

func (ts *Timestamp) Time() (tm time.Time) {
	if ts != nil {
		tm = (time.Time)(*ts)
	}
	return // tm
}

func (ts *Timestamp) UnmarshalText(data []byte) error {
	tm, err := time.Parse("2006-01-02T15:04:05-0700", string(data))
	if err != nil {
		return err
	}
	*(ts) = Timestamp(tm)
	return nil
}

// Instagram-scoped ID and username of the Instagram user who created the comment
type IGCommentFromUser struct {
	// IGSID of the Instagram user who created the IG Comment.
	ID string `json:"id"`
	// Username of the Instagram user who created the IG Comment.
	Username string `json:"username,omitempty"`
}

// ID and product type of the IG Media the comment was created on
type IGCommentMedia struct {
	// ID of the IG Media the comment was created on
	ID string `json:"id"`
	// Product type of the IG Media the comment was created on
	Type string `json:"media_product_type"`
}

// Represents an Instagram comment.
// https://developers.facebook.com/docs/instagram-api/reference/ig-comment/
type IGComment struct {
	// IG Comment ID.
	ID string `json:"id"`
	// From Instagram user who created the comment
	From IGCommentFromUser `json:"from"` // *IGUser
	// Indicates if comment has been hidden (true) or not (false).
	Hidden bool `json:"hidden,omitempty"`
	// Number of likes on the IG Comment.
	LikeCount int `json:"like_count,omitempty"`
	// IG Media upon which the IG Comment was made.
	Media *IGMedia `json:"media,omitempty"` // *IGCommentMedia
	// ID of the parent IG Comment if this comment was created on another IG Comment (i.e. a reply to another comment.
	ParentID string `json:"parent_id,omitempty"`
	// A list of replies (IG Comments) made on the IG Comment.
	// https://developers.facebook.com/docs/instagram-api/reference/ig-comment/replies
	Replies []*IGComment `json:"replies,omitempty"`
	// IG Comment text.
	Text string `json:"text"`
	// ISO 8601 formatted timestamp indicating when IG Comment was created.
	Date *Timestamp `json:"timestamp,omitempty"`
	// ID of IG User who created the IG Comment.
	// Only returned if the app user created the IG Comment,
	// otherwise username will be returned instead.
	User string `json:"user,omitempty"`
	// Username of Instagram user who created the IG Comment.
	Username string `json:"username,omitempty"`
}

// https://www.instagram.com/p/CkQ5eoHI8-k/c/17989946791606028/
func (e *IGComment) GetPermaLink() (href string, ok bool) {
	if e == nil || e.ID == "" {
		return // "", false
	}
	media := e.Media
	if media == nil {
		return // "", false
	}
	href = media.PermaLink
	if href == "" {
		if media.ShortCode == "" {
			return // "", false
		}
		href = "https://www.instagram.com/p" +
			path.Join("/", media.ShortCode, "/")
	}

	href = strings.TrimRight(href, "/") +
		path.Join("/c", e.ID, "/")

	return href, true
}

// IGMention media comment; event value
// https://developers.facebook.com/docs/graph-api/webhooks/reference/instagram/#mentions
type IGMention struct {
	// ID of media containing comment with mention.
	MediaID string `json:"media_id"`
	// ID of comment with mention.
	CommentID string `json:"comment_id"`
}

// IGMedia represents an Instagram album, photo, or video (uploaded video, live video, video created with the Instagram TV app, reel, or story).
// https://developers.facebook.com/docs/instagram-api/reference/ig-media
type IGMedia struct {

	// Caption. Excludes album children.
	// The @ symbol is excluded, unless the app user can perform admin-equivalent tasks on the Facebook Page
	// connected to the Instagram account used to create the caption.
	Caption string `json:"caption,omitempty"`

	// Count of comments on the media. Excludes comments on album child media and the media's caption. Includes replies on comments.
	CommentsCount int `json:"comments_count,omitempty"`

	// Media ID.
	ID string `json:"id,omitempty"`

	// // Instagram media ID.
	// // Used with Legacy Instagram API, now deprecated.
	// // Use id instead.
	// IGID string `json:"ig_id,omitempty"`

	// Indicates if comments are enabled or disabled. Excludes album children.
	IsCommentEnabled bool `json:"is_comment_enabled,omitempty"`

	// Reels only. If true, indicates the reel can appear in both the Feed and Reels tabs. If false, indicates the reel can only appear in the Reels tab.
	// Note that neither value indicates if the reel actually appears in the Reels tab, as the reel may not meet eligibilty requirements or have been selected by our algorithm. See reel specifications for eligibility critera.
	IsSharedToFeed bool `json:"is_shared_to_feed,omitempty"`

	// Count of likes on the media, including replies on comments. Excludes likes on album child media and likes on promoted posts created from the media.
	// If queried indirectly through another endpoint or field expansion:
	// v10.0 and older calls: The value is 0 if the media owner has hidden like counts.
	// v11.0+ calls: The like_count field is omitted if the media owner has hidden like counts.
	LikeCount int `json:"like_count,omitempty"`

	// Surface where the media is published. Can be AD, FEED, STORY or REELS.
	ProductType string `json:"media_product_type,omitempty"`

	// Media type. Can be CAROUSEL_ALBUM, IMAGE, or VIDEO.
	MediaType string `json:"media_type,omitempty"`

	// The URL for the media.
	// The media_url field is omitted from responses if the media contains copyrighted material or has been flagged for a copyright violation. Examples of copyrighted material can include audio on reels.
	MediaURL string `json:"media_url,omitempty"`

	// Instagram user ID who created the media.
	// Only returned if the app user making the query also created the media;
	// otherwise, username field is returned instead.
	Owner *IGCommentFromUser `json:"owner,omitempty"`

	// Permanent URL to the media.
	PermaLink string `json:"permalink,omitempty"`

	// Shortcode to the media.
	ShortCode string `json:"shortcode,omitempty"`

	// Media thumbnail URL. Only available on VIDEO media.
	ThumbnailURL string `json:"thumbnail_url,omitempty"`

	// ISO 8601-formatted creation date in UTC (default is UTC ±00:00).
	Timestamp *Timestamp `json:"timestamp,omitempty"`

	// Username of user who created the media.
	Username string `json:"username,omitempty"`

	// // Deprecated. Omitted from response.
	// VideoTitle string `json:"video_title,omitempty"`

	// Public edges can be returned through field expansion.

	// Children collection of IG Media objects on an album IG Media.
	Album []*IGMedia `json:"children,omitempty"`
	// Comments collection of IG Comments on an IG Media object.
	Comments []*IGComment `json:"comments,omitempty"`
	// // Insights social interaction metrics on an IG Media object.
	// Insights interface{} `json:"insights,omitempty"`
}

// https://www.instagram.com/p/CkQ5eoHI8-k/c/17989946791606028/
func (e *IGMedia) GetPermaLink() string {
	permaLink := e.PermaLink
	if permaLink == "" && e.ShortCode != "" {
		permaLink = "https://www.instagram.com/p" +
			path.Join("/", e.ShortCode, "/")
	}
	return permaLink
}

func (c *Client) fetchIGMedia(ctx context.Context, account *Page, media *IGMedia, fields ...string) error {

	// GET /17986679173617881?fields=caption,owner{name,username},media_type,media_product_type,media_url,permalink,shortcode,username,comments{timestamp,username,text}
	//
	// {
	//   "owner": {
	//     "name": "IGTest",
	//     "username": "webitel.chat.msgig",
	//     "id": "17841453641568741"
	//   },
	//   "media_type": "IMAGE",
	//   "media_product_type": "FEED",
	//   "media_url": "https://scontent.cdninstagram.com/v/t51.29350-15/312812634_860900711740574_5461841746268790086_n.webp?stp=dst-jpg&_nc_cat=110&ccb=1-7&_nc_sid=8ae9d6&_nc_ohc=0JhyisifsOsAX_2_Z6a&_nc_oc=AQl4dpZG1R8uxfH1qxBhKNAiK8LZlocR8byl9_nIx9XeWBBSnvpZex-fUt-tW359faU&_nc_ht=scontent.cdninstagram.com&edm=AEQ6tj4EAAAA&oh=00_AfACCaohjEJiaMC0pMwRQKl6nTJKIoe4bSfIzDnM2tPiHw&oe=6361E299",
	//   "permalink": "https://www.instagram.com/p/CkQ5eoHI8-k/",
	//   "shortcode": "CkQ5eoHI8-k",
	//   "username": "webitel.chat.msgig",
	//   "comments": {
	//     "data": [
	//       {
	//         "timestamp": "2022-10-28T16:31:00+0000",
	//         "username": "vkovalyshyn",
	//         "text": "Я коментую цей малюнок",
	//         "id": "17989946791606028"
	//       }
	//     ]
	//   },
	//   "id": "17986679173617881"
	// }

	if media == nil || media.ID == "" {
		return errors.BadRequest(
			"instagram.media.id.required",
			"instagram: GET media.id required but missing",
		)
	}

	if len(fields) == 0 {
		fields = []string{
			"id",
			"caption",
			"shortcode",
			"permalink",
			"media_url",
			"media_type",
			"media_product_type",
			"owner{id,username}",
		}
	}

	// TODO: USER_ACCESS_TOKEN
	// accessToken := account.Page.AccessToken
	query := c.requestForm(url.Values{
		"fields": {strings.Join(fields, ",")},
	}, account.AccessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, // GET
		"https://graph.facebook.com"+
			path.Join("/", c.Version, media.ID)+
			"?"+query.Encode(),
		nil,
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	var (
		res = struct {
			Error    *graph.Error `json:"error,omitempty"`
			*IGMedia              // Embedded (Anonymous)
		}{
			Error:   nil,
			IGMedia: media,
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err == nil && res.Error != nil {
		err = res.Error
	}

	if err != nil {
		return err
	}

	return nil
}

// UnifiedStoryMention
type StoryMention struct {
	// Media ID.
	ID string `json:"id"`
	// CDN Media content URL.
	Link string `json:"link"`
}

type IGStoryMention struct {
	// Message ID.
	ID string
	// UnifiedStoryMention
	Mention StoryMention
}

func (c *Client) fetchStoryMention(ctx context.Context, account *Page, mention *IGStoryMention) (*IGMedia, error) {

	// GET <MESSAGE_ID>?fields=story
	//
	// {
	// 	"story": {
	// 		"mention": {
	// 			"link": "<CDN_URL>",
	// 			"id": "<STORY_ID>"
	// 		}
	// 	},
	// 	"id": "<MESSAGE_ID>"
	// }
	// https://developers.facebook.com/docs/messenger-platform/instagram/features/story-mention/#example-request-to-retrieve-story-mention-via-conversation-api
	mediaURL, err := url.ParseRequestURI(mention.Mention.Link)
	if err != nil {
		return nil, err
	}
	mention.Mention.ID = mediaURL.Query().Get("asset_id")
	story := &IGMedia{
		ID: mention.Mention.ID,
	}
	err = c.fetchIGMedia(ctx, account, story,
		"caption",    // as mention[ed] message text
		"media_type", // Content MIMETYPE: CAROUSEL_ALBUM, IMAGE, or VIDEO
		// "media_product_type", // Surface where the media is published. Can be AD, FEED, STORY or REELS.
		"permalink",
	)
	// if err != nil {
	// 	return story, err
	// }
	return story, err
}

func (c *Client) fetchIGComment(ctx context.Context, account *Page, comment *IGComment, fields ...string) error {

	if comment == nil || comment.ID == "" {
		return errors.BadRequest(
			"instagram.comment.id.required",
			"instagram: GET comment.id required but missing",
		)
	}

	if len(fields) == 0 {
		fields = []string{
			"id",
			"from",
			"text",
			"timestamp",
			"parent_id",
			"media{id,caption,media_type,media_product_type,shortcode,permalink}",
		}
	}

	// GET /17989946791606028?fields=timestamp,username,text

	// Permalink: https://www.instagram.com/p/${media.shortcode}/c/${comment.id}/
	// TODO: USER_ACCESS_TOKEN
	// accessToken := account.Page.AccessToken
	query := c.requestForm(url.Values{
		"fields": {strings.Join(fields, ",")},
	}, account.AccessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, // GET
		"https://graph.facebook.com"+
			path.Join("/", c.Version, comment.ID)+
			"?"+query.Encode(),
		nil,
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	var (
		res = struct {
			Error      *graph.Error `json:"error,omitempty"`
			*IGComment              // Embedded (Anonymous)
		}{
			Error:     nil,
			IGComment: comment,
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err == nil && res.Error != nil {
		err = res.Error
	}

	if err != nil {
		return err
	}

	return nil
}

// https://developers.facebook.com/docs/instagram-api/reference/ig-user/mentioned_media#reading
// GET /17841453641568741?fields=mentioned_media.media_id(17940733142416889){id,caption,media_url,media_type,media_product_type,permalink,owner,username}
func (c *Client) fetchIGMentionMedia(ctx context.Context, account *Page, media *IGMedia, fields ...string) error {

	// // GET /17841453641568741?fields=mentioned_media.media_id(17940733142416889){id,caption,media_url,media_type,media_product_type,permalink,owner,username}
	//
	// {
	// 	"mentioned_comment": {
	// 		"id": "17958981301970930",
	// 		"media": {
	// 			"permalink": "https://www.instagram.com/p/CkX46fTsGVE/",
	// 			"id": "17940733142416889"
	// 		},
	// 		"timestamp": "2022-10-31T09:37:24+0000",
	// 		"like_count": 0,
	// 		"text": "@webitel.chat.msgig я тебе ще й в коментарі згадую"
	// 	},
	// 	"id": "17841453641568741"
	// }

	if media == nil || media.ID == "" {
		return errors.BadRequest(
			"instagram.mention.media.id.required",
			"instagram: GET mention.media.id required but missing",
		)
	}

	if len(fields) == 0 {
		fields = []string{
			"id",
			"caption",
			"owner",
			"username",
			"permalink",
			"media_url",
			"media_type",
			"media_product_type",
		}
	}

	// TODO: USER_ACCESS_TOKEN
	// accessToken := account.Page.AccessToken
	query := c.requestForm(url.Values{
		"fields": {"mentioned_media.media_id(" + media.ID + "){" + strings.Join(fields, ",") + "}"},
	}, account.AccessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, // GET
		"https://graph.facebook.com"+
			path.Join("/", c.Version, account.Page.Instagram.ID)+
			"?"+query.Encode(),
		nil,
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	var (
		res = struct {
			Error *graph.Error `json:"error,omitempty"`
			Media *IGMedia     `json:"mentioned_media,omitempty"`
		}{
			Error: nil,
			Media: media,
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err == nil && res.Error != nil {
		err = res.Error
	}

	if err != nil {
		return err
	}

	return nil
}

// https://developers.facebook.com/docs/instagram-api/reference/ig-user/mentioned_comment#reading1
// GET /17841453641568741?fields=mentioned_comment.comment_id(17958981301970930){id,media{permalink},timestamp,like_count,text}
func (c *Client) fetchIGMentionComment(ctx context.Context, account *Page, comment *IGComment, fields ...string) error {

	// GET /17841453641568741?fields=mentioned_comment.comment_id(17958981301970930){id,media{permalink},timestamp,like_count,text}
	//
	// {
	// 	"mentioned_comment": {
	// 		"id": "17958981301970930",
	// 		"media": {
	// 			"permalink": "https://www.instagram.com/p/CkX46fTsGVE/",
	// 			"id": "17940733142416889"
	// 		},
	// 		"timestamp": "2022-10-31T09:37:24+0000",
	// 		"like_count": 0,
	// 		"text": "@webitel.chat.msgig я тебе ще й в коментарі згадую"
	// 	},
	// 	"id": "17841453641568741"
	// }

	if comment == nil || comment.ID == "" {
		return errors.BadRequest(
			"instagram.mention.comment.id.required",
			"instagram: GET mention.comment.id required but missing",
		)
	}

	if len(fields) == 0 {
		// https://developers.facebook.com/docs/instagram-api/reference/ig-user/mentioned_comment#fields
		// https://developers.facebook.com/docs/instagram-api/reference/ig-comment#fields
		fields = []string{
			"id",
			"from",
			"text",
			// "like_count",
			"timestamp",
			"media{permalink}",
		}
	}

	// GET /17989946791606028?fields=timestamp,username,text

	// Permalink: https://www.instagram.com/p/${media.shortcode}/c/${comment.id}/
	// TODO: USER_ACCESS_TOKEN
	// accessToken := account.Page.AccessToken
	query := c.requestForm(url.Values{
		"fields": {"mentioned_comment.comment_id(" + comment.ID + "){" + strings.Join(fields, ",") + "}"},
	}, account.AccessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, // GET
		"https://graph.facebook.com"+
			path.Join("/", c.Version, account.Page.Instagram.ID)+
			"?"+query.Encode(),
		nil,
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	var (
		res = struct {
			Error   *graph.Error `json:"error,omitempty"`
			Comment *IGComment   `json:"mentioned_comment,omitempty"`
		}{
			Error:   nil,
			Comment: comment,
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err == nil && res.Error != nil {
		err = res.Error
	}

	if err != nil {
		return err
	}

	return nil
}

func (c *Client) getIGComment(ctx context.Context, account *Page, commentId string) (*IGComment, error) {
	// GET /17989946791606028?fields=timestamp,username,text

	// Permalink: https://www.instagram.com/p/${media.shortcode}/c/${comment.id}/
	// TODO: USER_ACCESS_TOKEN
	// accessToken := account.Page.AccessToken
	query := c.requestForm(url.Values{
		"fields": {"id,parent_id,media{id,permalink,shortcode},timestamp,from,text"},
	}, account.AccessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, // GET
		"https://graph.facebook.com"+
			path.Join("/", c.Version, commentId)+
			"?"+query.Encode(),
		nil,
	)

	if err != nil {
		return nil, err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var (
		res = struct {
			Error     *graph.Error `json:"error,omitempty"`
			IGComment              // Embedded (Anonymous)
		}{
			// Alloc
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err == nil && res.Error != nil {
		err = res.Error
	}

	if err != nil {
		return nil, err
	}

	return &res.IGComment, nil
}

func (c *Client) WebhookInstagram(batch []*messenger.Entry) {

	var (
		err error
		on  = "instagram.onUpdate"
	)
	for _, entry := range batch {
		if len(entry.Messaging) != 0 {
			// Array containing one messaging object.
			// Note that even though this is an array,
			// it will only contain one messaging object.
			// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#entry
			for _, event := range entry.Messaging {
				if event.Message != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
					on = "instagram.onMessage"
					err = c.WebhookMessage(event)
				} else if event.Postback != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
					on = "instagram.onPostback"
					err = c.WebhookPostback(event)
				} // else {
				// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#event_list
				// }
			}

		} else if vs := entry.Changes; len(vs) != 0 {

			for _, e := range vs {
				switch e.Field {
				// Notifies you when an Instagram User comments on a media object that you own.
				// https://developers.facebook.com/docs/graph-api/webhooks/reference/instagram/#comments
				case "comments":

					hook := c.hookIGMediaComment
					// c.onInstagramComment
					if hook == nil {
						c.Gateway.Log.Warn("instagram.onComment",
							slog.String("error", "update: instagram{comments} is disabled"),
						)
						break // switch // (200) OK
					}

					var comment IGComment
					err = e.GetValue(&comment)
					if err != nil {
						c.Gateway.Log.Error("instagram.onComment",
							slog.Any("error", err),
						)
						break // switch // (200) OK
					}
					// Handle update event
					hook(entry.ObjectID, &comment)

				// Notifies you when an Instagram User @mentions you in a comment or caption on a media object that you do not own.
				// https://developers.facebook.com/docs/graph-api/webhooks/reference/instagram/#mentions
				case "mentions":

					hook := c.hookIGMediaMention
					// c.onInstagramMention
					if hook == nil {
						c.Gateway.Log.Warn("instagram.onMention",
							slog.String("error", "update: instagram{mentions} is disabled"),
						)
						break // switch // (200) OK
					}

					var mention IGMention
					err := e.GetValue(&mention)
					if err != nil {
						c.Gateway.Log.Error("instagram.onMention",
							slog.Any("error", err),
						)
						break
					}
					// Handle update event
					hook(entry.ObjectID, &mention)

				default:
					c.Gateway.Log.Warn("instagram.onUpdate",
						slog.String("field", e.Field),
						slog.String("error", "update: instagram{"+e.Field+"} field is unknown"),
					)
				}
			}

			// } else if len(entry.Standby) != 0 {
			// 	// Array of messages received in the standby channel.
			// 	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/standby
			// 	for _, event := range entry.Standby {
			//
			// 	}
			// }

		} else {
			on = "instagram.onUpdate"
			err = errors.BadRequest(
				"messenger.update.not_supported",
				"instagram: update event type not supported",
			)
		}

		if err != nil {
			re := errors.FromError(err)
			c.Gateway.Log.Error(on,
				slog.String("error", re.Detail),
			)
			err = nil
			// continue
		}
	}
}

// IGSID, as an [I]nta[g]ram-[s]coped [ID] recipient; Instagram Owner of the IGMedia, which was just commented
// comment, as an update event argument
func (c *Client) onIGMediaComment(IGSID string, comment *IGComment) {
	// NOTE: Has partial content! For more info see comments.value definition
	// https://developers.facebook.com/docs/graph-api/webhooks/reference/instagram/#comments

	// Resolve comment's Instagram Account ID related TO
	account := c.instagram.getPage(IGSID)
	if account == nil {
		c.Gateway.Log.Error("instagram.onComment",
			slog.String("error", "instagram: page not found"),
			slog.String("igsid", IGSID),
		)
		return
	}
	// GET more comment's data to be able to generate valid comment permalink
	ctx := context.TODO()
	err := c.fetchIGComment(ctx, account, comment,
		// "id",
		"media{id,caption,media_type,media_product_type,shortcode,permalink}",
	)
	if err != nil {
		c.Gateway.Log.Error("instagram.onComment",
			slog.Any("error", err),
			slog.String("igsid", IGSID),
		)
		return
	}

	commentLink, ok := comment.GetPermaLink()
	if !ok {
		c.Gateway.Log.Warn("instagram.onComment",
			slog.String("error", "instagram: not enough data to generate a comment permalink"),
			slog.String("igsid", IGSID),
		)
		return
	}

	sender := comment.From
	instagram := account.Page.Instagram

	userPSID := sender.ID // [P]age-[s]coped [ID]
	// pageASID := instagram.ID // [A]pp-[s]coped [ID] -or- [I]nsta[G]ram-[s]coped [ID]

	// channel, err := c.getInternalThread(
	// 	ctx, pageASID, userPSID,
	// )
	contact := bot.Account{
		ID: 0, // LOOKUP
		// FirstName: sender.Name, // sender.FirstName,
		// LastName:  sender.LastName,
		Username: sender.Username,
		// NOTE: This is the [P]age-[S]coped User [ID]
		// For the same Facebook User, but different Pages
		// this value differs
		Channel: "instagram",
		Contact: sender.ID,
	}

	// GET Chat
	chatID := userPSID // .sender.id
	channel, err := c.Gateway.GetChannel(
		ctx, chatID, &contact,
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		// http.Error(reply, re.Detail, (int)(re.Code))
		return // nil, re // 503 Bad Gateway
	}

	// IGID := instagram.ID
	sendMsg := &chat.Message{
		Type: "text",
		// Text: "#comment",
		Text: "[@comment]: " + comment.Text + "\n" + commentLink,
		Variables: map[string]string{
			// Comment Data
			paramIGCommentText: comment.Text,
			paramIGCommentLink: commentLink,
		},
	}

	if channel.IsNew() {
		// VIA: Facebook Page
		sendMsg.Variables[paramFacebookPage] = account.Page.ID
		sendMsg.Variables[paramFacebookName] = account.Page.Name
		// VIA: Instagram Page
		sendMsg.Variables[paramInstagramPage] = instagram.ID
		sendMsg.Variables[paramInstagramUser] = instagram.Username
		// autobind to channel.properties
		envar := channel.Properties.(map[string]string)
		// MUST: channel.Properties.(map[string]string{externalChatID:})
		for h, vs := range sendMsg.Variables {
			envar[h] = vs
		}
	}

	update := bot.Update{
		Title:   channel.Title,
		Chat:    channel,
		User:    &channel.Account,
		Message: sendMsg,
	}

	// Forward Instagram comment update as an internal message
	err = c.Gateway.Read(ctx, &update)

	if err != nil {
		c.Gateway.Log.Error("instagram.onComment",
			slog.Any("error", err),
		)
		// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
		return // err // 502 Bad Gateway
	}

	c.Gateway.Log.Debug("instagram.onComment",
		slog.Any("sender", log2.SlogObject(comment.From)),
		slog.String("comment", comment.Text),
		slog.String("permalink", commentLink),
		// Authenticated VIA Facebook Page
		slog.Any("facebook", log2.SlogObject(IGCommentFromUser{
			ID: account.ID, Username: account.Name,
		})),
		// Comment FOR Instagram Account
		slog.Any("instagram", log2.SlogObject(IGCommentFromUser{
			ID: instagram.ID, Username: instagram.Username,
		})),
	)
}

// account, as a recipient; Instagram Page, which was just mentioned
// metion, as an update args
func (c *Client) onIGMediaMention(IGSID string, mention *IGMention) {
	// Resolve comment's Instagram Account ID related TO
	account := c.instagram.getPage(IGSID)
	if account == nil {
		c.Gateway.Log.Error("instagram.onMention",
			slog.String("error", "instagram: page account not found"),
			slog.String("igsid", IGSID),
		)
		return
	}

	var (
		ctx    = context.TODO()
		sender IGCommentFromUser
		// enVars = make(map[string]string)
		// captIn = (mention.CommentID == "")
		// Default: IGMedia @mention in .Caption
		media = IGMedia{
			ID: mention.MediaID,
		}
		mentionText string
		mentionLink string
	)
	// Media caption @mention ?
	if mention.CommentID == "" {
		// https://developers.facebook.com/docs/instagram-api/reference/ig-user/mentioned_media#reading
		// GET /17841453641568741?fields=mentioned_media.media_id(17940733142416889){id,caption,media_url,media_type,media_product_type,permalink,owner,username}
		err := c.fetchIGMentionMedia(ctx, account, &media)
		// "id",
		// "caption",
		// "shortcode",
		// "permalink",
		// "media_url",
		// "media_type",
		// "media_product_type",
		// "owner{id,username}",
		if err != nil {
			c.Gateway.Log.Error("instagram.onMention",
				slog.Any("error", err),
				slog.String("igsid", IGSID),
			)
			return
		}

		mentionText = media.Caption
		mentionLink = media.PermaLink

		if from := media.Owner; from != nil {
			sender.ID = from.ID
			sender.Username = from.Username
		}
		if sender.ID == "" {
			sender.ID = media.ID // mentioned_media.id !!!
		}
		if sender.Username == "" {
			sender.Username = media.Username
		}

	} else {

		comment := IGComment{
			ID:    mention.CommentID,
			Media: &media,
		}

		// https://developers.facebook.com/docs/instagram-api/reference/ig-user/mentioned_comment#reading
		// GET /17841453641568741?fields=mentioned_comment.comment_id(17958981301970930){id,media{permalink},timestamp,like_count,text}
		err := c.fetchIGMentionComment(ctx, account, &comment)
		// "id",
		// "from",
		// "text",
		// "timestamp",
		// "parent_id",
		// "media{id,caption,media_type,media_product_type,shortcode,permalink}",

		if err != nil {
			c.Gateway.Log.Error("instagram.onMention",
				slog.Any("error", err),
				slog.String("igsid", IGSID),
			)
			return
		}

		mentionText = comment.Text
		mentionLink, _ = comment.GetPermaLink()

		// if from := comment.From; from != nil {
		sender.ID = comment.From.ID
		sender.Username = comment.From.Username
		// }
		if sender.ID == "" {
			sender.ID = comment.ID // mentioned_comment.id !!!
		}
		if sender.Username == "" {
			sender.Username = comment.Username
		}

	}

	// sender := comment.From
	instagram := account.Page.Instagram

	userPSID := sender.ID // [P]age-[s]coped [ID]
	// pageASID := instagram.ID // [A]pp-[s]coped [ID] -or- [I]nsta[G]ram-[s]coped [ID]

	// channel, err := c.getInternalThread(
	// 	ctx, pageASID, userPSID,
	// )
	contact := bot.Account{
		ID: 0, // LOOKUP
		// FirstName: sender.Name, // sender.FirstName,
		// LastName:  sender.LastName,
		Username: sender.Username,
		// NOTE: This is the [P]age-[S]coped User [ID]
		// For the same Facebook User, but different Pages
		// this value differs
		Channel: "instagram",
		Contact: sender.ID,
	}

	// GET Chat
	chatID := userPSID // .sender.id
	channel, err := c.Gateway.GetChannel(
		ctx, chatID, &contact,
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		// http.Error(reply, re.Detail, (int)(re.Code))
		return // nil, re // 503 Bad Gateway
	}

	// IGID := instagram.ID
	sendMsg := &chat.Message{
		Type: "text",
		// Text: "#mention",
		Text: "[@mention]: " + mentionText + "\n" + mentionLink,
		Variables: map[string]string{
			// Mention Data
			paramIGMentionText: mentionText,
			paramIGMentionLink: mentionLink,
		},
	}

	if channel.IsNew() {
		// VIA: Facebook Page
		sendMsg.Variables[paramFacebookPage] = account.Page.ID
		sendMsg.Variables[paramFacebookName] = account.Page.Name
		// VIA: Instagram Page
		sendMsg.Variables[paramInstagramPage] = instagram.ID
		sendMsg.Variables[paramInstagramUser] = instagram.Username
		// autobind to channel.properties
		envar := channel.Properties.(map[string]string)
		// MUST: channel.Properties.(map[string]string{externalChatID:})
		for h, vs := range sendMsg.Variables {
			envar[h] = vs
		}
	}

	update := bot.Update{
		Title:   channel.Title,
		Chat:    channel,
		User:    &channel.Account,
		Message: sendMsg,
	}

	// Forward Instagram comment update as an internal message
	err = c.Gateway.Read(ctx, &update)

	if err != nil {
		c.Gateway.Log.Error("instagram.onMention",
			slog.Any("error", err),
		)
		// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
		return // err // 502 Bad Gateway
	}

	c.Gateway.Log.Debug("instagram.onMention",
		slog.Any("sender", log2.SlogObject(sender)),
		slog.String("mentionText", mentionText),
		slog.String("permalink", mentionLink),
		// Authenticated VIA Facebook Page
		slog.Any("facebook", log2.SlogObject(IGCommentFromUser{
			ID: account.ID, Username: account.Name,
		})),
		// Comment FOR Instagram Account
		slog.Any("instagram", log2.SlogObject(IGCommentFromUser{
			ID: instagram.ID, Username: instagram.Username,
		})),
	)
}

func (c *Client) onIGStoryMention(IGSID string, mention *IGStoryMention) {
	// NOTE: stub function to enable instagram.story_mention processing ...
}
