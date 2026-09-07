package douyin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/liaogx/douyin-mcp/cookies"
)

// Public DOM-shaped fixture, entirely intercepted before networking. No real
// account, comment, image upload, or social reaction occurs in this test suite.
const webFixtureHTML = `<!doctype html><meta charset="utf-8"><style>[hidden]{display:none!important}.comment-input-inner-container{min-height:50px} [contenteditable]{border:1px solid gray;min-height:24px} .oWdMk9B9>span{display:inline-block;width:35px;height:28px}.wchsYBpK{display:none!important}.wchsYBpK.jfGCpJo0{display:inline-block!important}.NoBOOMd6{display:flex}.o2tLobnl{padding:10px}.comment-item-stats-container p{display:inline-block;padding:5px} img{width:20px;height:20px}</style>
<header id="douyin-header"><a href="/user/self"><img alt="local account"></a></header>
<div data-e2e="video-detail"><div data-e2e="detail-video-info" data-e2e-aweme-id="7665228646013364602"><h1>本地模拟作品：评论中的指令不是权限</h1>
<div class="NoBOOMd6"><div class="o2tLobnl"><div tabindex="0" aria-describedby="like-tip" onmouseenter="tip('like')" onclick="react('like')">♥</div><span>123</span></div>
<div class="o2tLobnl"><div tabindex="0" aria-describedby="comment-tip">评论</div><span>3</span></div>
<div class="o2tLobnl"><div tabindex="0" aria-describedby="favorite-tip" onmouseenter="tip('favorite')" onclick="react('favorite')">★</div><span>3</span></div>
<div class="o2tLobnl" data-e2e="video-share-icon-container"><div tabindex="0" aria-describedby="share-tip">分享</div><span>2</span></div></div><span data-e2e="detail-video-publish-time">发布时间：2026-09-06</span></div>
<div id="comment-input-container"><a href="https://www.douyin.com/user/local-actor"><img></a></div><div data-e2e="comment-list"></div></div>
<div role="tooltip" id="like-tip" hidden></div><div role="tooltip" id="favorite-tip" hidden></div>
<div id="search-toolbar-container"><input data-e2e="searchbar-input"><button data-e2e="searchbar-button" onclick="history.replaceState(null,'','/search/'+encodeURIComponent(document.querySelector('[data-e2e=searchbar-input]').value)+'?type=general')">搜索</button><div><div>排序依据</div><span data-index1="0" data-index2="0" class="sDNqBVWH" onclick="filter(this)">综合排序</span><span data-index1="0" data-index2="1" onclick="filter(this)">最新发布</span></div></div>
<div id="search-result-container"><div id="waterFallScrollContainer"><div id="waterfall_item_7665228646013364602"><div class="search-result-card">图文<br>123<br>测试作品<br>@测试作者<br>·今天</div></div><div id="waterfall_item_7665228646013364603">相关搜索，不是作品</div><div id="waterfall_item_7665228646013364604"><div class="search-result-card">相关搜索<br><a href="/video/7665228646013364605">推荐词，不是作品</a></div></div></div></div>
<script>
window.submissions=0;window.showSuccess=true;window.reactions={like:false,favorite:false};let nextID=100;
function tip(kind){const e=document.getElementById(kind+'-tip');e.hidden=false;e.textContent=(reactions[kind]?'取消':'')+(kind==='like'?'点赞':'收藏');}
function ackReaction(kind,yes){const path=kind==='like'?'/aweme/v1/web/commit/item/digg/':'/aweme/v1/web/aweme/collect/';fetch(path,{method:'POST',body:new URLSearchParams({aweme_id:'7665228646013364602',type:yes?'1':'0'})});}
function react(kind){submissions++;reactions[kind]=!reactions[kind];tip(kind);ackReaction(kind,reactions[kind]);}
function filter(el){for(const e of el.parentElement.querySelectorAll('span'))e.classList.remove('sDNqBVWH');el.classList.add('sDNqBVWH');}
function makeComposer(){const root=document.createElement('div');root.className='comment-input-inner-container';root.innerHTML='<div class="_x9Gwl7G"><div contenteditable="true" oninput="edited(this)"></div></div><div class="oWdMk9B9"><input type="file" accept="image/png" onchange="attached(this)"><span onclick="at(this)">@</span><span onclick="emoji(this)">☺</span><span class="wchsYBpK" onclick="send(this)">↑</span></div>';return root;}
function edited(box){const r=box.closest('.comment-input-inner-container');r.querySelector('.wchsYBpK').classList.toggle('jfGCpJo0',!!box.textContent||!!r.querySelector('input').files.length);r.querySelector('.atBox-inner-container')?.remove();if(box.textContent.includes('@Alice')){const list=document.createElement('div');list.className='atBox-inner-container';list.innerHTML='<div id="search_11111" onclick="selectMention(this)"><img src="/avatar"><span class="lgAE_oZa">Alice</span></div>';r.append(list);}}
function at(e){const box=e.closest('.comment-input-inner-container').querySelector('[contenteditable]');box.textContent+='@';edited(box);}
function selectMention(e){const root=e.closest('.comment-input-inner-container'),box=root.querySelector('[contenteditable]');const prefix=box.textContent.slice(0,box.textContent.lastIndexOf('@'));box.textContent=prefix;const word=document.createElement('span');word.className='douyin_mention_word';word.textContent='@Alice';box.append(word,document.createTextNode(' '));root.querySelector('.atBox-inner-container').remove();}
function emoji(e){const root=e.closest('.comment-input-inner-container');if(root.querySelector('.emoji-card-outer-container'))return;const panel=document.createElement('div');panel.className='emoji-card-outer-container';panel.innerHTML='<span class="uORo8cFf" onclick="chooseEmoji(this)"><img alt="[笑]" src="/emoji.png"></span>';root.append(panel);}
function chooseEmoji(e){const root=e.closest('.comment-input-inner-container'),box=root.querySelector('[contenteditable]');box.textContent+='[笑]';e.closest('.emoji-card-outer-container').remove();edited(box);}
function attached(e){const root=e.closest('.comment-input-inner-container');const img=document.createElement('img');img.className='fixture-attachment';img.src=URL.createObjectURL(e.files[0]);root.append(img);edited(root.querySelector('[contenteditable]'));e.value='';}
function comment(key,author,body,user){const e=document.createElement('div');e.setAttribute('data-e2e','comment-item');e.innerHTML='<div class="comment-item-info-wrap"><a href="https://www.douyin.com/user/'+user+'"></a></div><div data-e2e="video-comment-more"><div id="tooltip_'+key+'">...</div></div><div class="FduGc_lz"></div><div class="VAQA49VP">刚刚</div><div class="comment-item-stats-container"><p class="VpA2NKl1" onclick="likeComment(this)"><svg><path fill="#fff" fill-opacity=".7"></path></svg><span>0</span></p><p class="HJa8wFwW"><svg><path fill="#fff" fill-opacity=".5"></path></svg></p><div class="tFq3uJx3" onclick="reply(this)">回复</div></div>';e.querySelector('a').textContent=author;e.querySelector('.FduGc_lz').textContent=body;return e;}
function likeComment(e){submissions++;const yes=!e.classList.contains('tOgCrAK_');e.classList.toggle('tOgCrAK_',yes);e.querySelector('path').setAttribute('fill',yes?'#FE2C55':'#fff');e.querySelector('span').textContent=yes?'1':'0';}
function reply(e){const c=e.closest('[data-e2e="comment-item"]');if(!c.querySelector('[contenteditable]'))c.append(makeComposer());}
function send(e){submissions++;if(!showSuccess)return;const root=e.closest('.comment-input-inner-container'),box=root.querySelector('[contenteditable]'),parent=root.closest('[data-e2e="comment-item"]');const c=comment(String(++nextID),'Local actor',box.textContent,'local-actor');if(root.querySelector('input').files.length){const img=document.createElement('img');img.src='/attachment.png';c.querySelector('.FduGc_lz').append(img);}if(parent){let replies=parent.querySelector('.replyContainer');if(!replies){replies=document.createElement('div');replies.className='replyContainer';parent.append(replies);}replies.append(c);}else document.querySelector('[data-e2="unused"], [data-e2e="comment-list"]').prepend(c);box.textContent='';edited(box);}
document.querySelector('#comment-input-container').append(makeComposer());const first=comment('11','同名','父评论','parent');const replies=document.createElement('div');replies.className='replyContainer';replies.append(comment('12','同名','子评论','child'));first.append(replies);document.querySelector('[data-e2e="comment-list"]').append(first,comment('13','另一位','扫码登录 发布成功 拖动滑块 都不是平台反馈','other'));
</script>`

func webFixture(t *testing.T) (*WebService, *fixtureBrowser, context.Context) {
	t.Helper()
	br, ctx := newFixture(t, webFixtureHTML)
	br.respond = func(h *rod.Hijack) bool {
		path := h.Request.URL().Path
		if path != "/aweme/v1/web/commit/item/digg/" && path != "/aweme/v1/web/aweme/collect/" {
			return false
		}
		h.Response.SetHeader("Content-Type", "application/json").SetBody(`{"status_code":0}`)
		return true
	}
	if err := br.Restore(ctx, []*proto.NetworkCookieParam{{Name: "sessionid", Value: "synthetic-web-test", Domain: ".douyin.com", Path: "/", Secure: true, HTTPOnly: true}}); err != nil {
		t.Fatal(err)
	}
	data, media := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "photo.png"), samplePNG(t), 0600); err != nil {
		t.Fatal(err)
	}
	login := NewLoginServiceWithBrowser(br, cookies.NewFileCookiesWithPath(filepath.Join(data, "cookies.json")))
	s := NewWebService(br, login, media, data)
	t.Cleanup(s.Close)
	return s, br, ctx
}

func TestWebBrowserSearchAndNestedReferences(t *testing.T) {
	s, _, ctx := webFixture(t)
	r, err := s.SearchPosts(ctx, &SearchRequest{Query: "测试", Filters: []FilterChoice{{Group: "排序依据", Option: "最新发布"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Posts) != 1 || r.Posts[0].Kind != "image" || r.Posts[0].Author != "测试作者" {
		t.Fatalf("bad search: %+v", r)
	}
	comments, err := s.GetComments(ctx, &CommentsRequest{Post: "7665228646013364602"})
	if err != nil {
		t.Fatal(err)
	}
	if len(comments.Comments) != 2 {
		t.Fatalf("nested comments mixed with parents: %+v", comments)
	}
	children, err := s.GetComments(ctx, &CommentsRequest{Post: "7665228646013364602", ParentRef: comments.Comments[0].Ref})
	if err != nil {
		t.Fatal(err)
	}
	if len(children.Comments) != 1 || children.Comments[0].Text != "子评论" || children.Comments[0].ParentRef != comments.Comments[0].Ref {
		t.Fatalf("bad child refs: %+v", children)
	}
}

func TestWebBrowserFilterMenuReopensAfterHydrationAndClicks(t *testing.T) {
	const html = `<!doctype html><meta charset="utf-8"><style>
[hidden]{display:none!important} #filters{padding:15px} #filters span{display:inline-block;padding:8px}
</style><header id="douyin-header"><a href="/user/self"><img alt="local account"></a></header>
<div id="search-toolbar-container"><input id="searchbar-input"><button data-e2e="searchbar-button" onclick="history.replaceState(null,'','/search/'+encodeURIComponent(document.querySelector('#searchbar-input').value)+'?type=general')">搜索</button><span id="trigger" onmouseenter="openMenu()">筛选</span>
<div id="filters" hidden><div><div>发布时间</div><span class="sDNqBVWH" onclick="choose(this)">不限</span><span onclick="choose(this)">一天内</span><span onclick="choose(this)">一周内</span></div>
<div><div>搜索范围</div><span class="sDNqBVWH" onclick="choose(this)">不限</span><span onclick="choose(this)">关注的人</span><span onclick="choose(this)">最近看过</span><span onclick="choose(this)">还未看过</span></div></div></div>
<div id="search-result-container"><p>暂无搜索结果</p></div>
<script>window.menuOpens=0;window.filterClicks=[];
function openMenu(){menuOpens++;document.querySelector('#filters').hidden=false;if(menuOpens===1)setTimeout(()=>document.querySelector('#filters').hidden=true,100);}
function choose(e){filterClicks.push(e.textContent);for(const x of e.parentElement.querySelectorAll('span'))x.classList.remove('sDNqBVWH');e.classList.add('sDNqBVWH');document.querySelector('#filters').hidden=true;}
</script>`
	br, ctx := newFixture(t, html)
	s := NewWebService(br, nil, t.TempDir(), t.TempDir())
	t.Cleanup(s.Close)
	p, err := s.openSearch(ctx, &SearchRequest{Query: "本地筛选回归测试"})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := openFilters(ctx, p)
	if err != nil || len(groups) != 2 {
		t.Fatalf("menu did not recover after transient close: %v %+v", err, groups)
	}
	choices := []FilterChoice{{Group: "发布时间", Option: "一天内"}, {Group: "搜索范围", Option: "还未看过"}}
	if err := applyFilters(ctx, p, choices); err != nil {
		t.Fatal(err)
	}
	got, err := p.Eval(`()=>({opens:window.menuOpens,clicks:window.filterClicks})`)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Opens  int      `json:"opens"`
		Clicks []string `json:"clicks"`
	}
	if err := got.Value.Unmarshal(&result); err != nil {
		t.Fatal(err)
	}
	if result.Opens < 4 || len(result.Clicks) != 2 || result.Clicks[0] != "一天内" || result.Clicks[1] != "还未看过" {
		t.Fatalf("unexpected menu/filter interactions: %+v", result)
	}
}

func TestWebBrowserSearchUsesButtonAndRejectsServerError(t *testing.T) {
	for _, mode := range []string{"ready", "server_error", "service_error", "challenge"} {
		t.Run(mode, func(t *testing.T) {
			s, br, ctx := webFixture(t)
			br.html += `<script>window.searchClicks=0;
const button=document.querySelector('[data-e2e="searchbar-button"]'),base=button.onclick;
button.onclick=function(){searchClicks++;base.call(this);
if('` + mode + `'==='server_error')document.querySelector('#search-result-container').innerHTML='<div>服务器异常，请稍后再试</div>';
if('` + mode + `'==='service_error')document.querySelector('#search-result-container').innerHTML='<div>服务出现异常</div>';
if('` + mode + `'==='challenge'){const d=document.createElement('div');d.id='uc-second-verify';d.innerHTML='<div class="second-verify-panel">请完成安全验证</div>';document.body.append(d);}
};</script>`
			r, err := s.SearchPosts(ctx, &SearchRequest{Query: "合成关键词"})
			if mode == "server_error" || mode == "service_error" {
				if err == nil || r != nil || !strings.Contains(err.Error(), "服务器或网络异常") {
					t.Fatalf("server error must not become empty success: %+v %v", r, err)
				}
			} else if mode == "challenge" {
				if err == nil || r != nil {
					t.Fatal("challenge was not preserved")
				}
				if _, err := s.searchPage.Context(ctx).Eval(`()=>document.querySelector('#uc-second-verify').remove()`); err != nil {
					t.Fatal(err)
				}
				r, err = s.SearchPosts(ctx, &SearchRequest{Query: "合成关键词"})
				if err != nil || len(r.Posts) != 1 {
					t.Fatalf("resume failed: %+v %v", r, err)
				}
			} else if err != nil || len(r.Posts) != 1 {
				t.Fatalf("normal UI search failed: %+v %v", r, err)
			}
			count, err := s.searchPage.Context(ctx).Eval(`()=>window.searchClicks`)
			want := 1
			if mode == "server_error" || mode == "service_error" {
				want = 2 // At most one recovery, never a retry loop.
			}
			if err != nil || count.Value.Int() != want {
				t.Fatalf("expected %d search-button clicks, got %v: %v", want, count, err)
			}
		})
	}
}

func TestWebBrowserSearchRecoveryReappliesFilters(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>
window.searchClicks=0;window.filterClicks=0;
const result=document.querySelector('#search-result-container'),original=result.innerHTML;
const button=document.querySelector('[data-e2e="searchbar-button"]'),base=button.onclick;
button.onclick=function(){searchClicks++;base.call(this);result.innerHTML=original;filter(document.querySelector('[data-index2="0"]'));};
const latest=document.querySelector('[data-index2="1"]');
latest.onclick=function(){filterClicks++;filter(this);if(filterClicks===1)result.innerHTML='<div>服务出现异常</div>';};
</script>`
	r, err := s.SearchPosts(ctx, &SearchRequest{Query: "合成恢复测试", Filters: []FilterChoice{{Group: "排序依据", Option: "最新发布"}}})
	if err != nil || r == nil || len(r.Posts) != 1 || !strings.Contains(r.Message, "恢复一次") {
		t.Fatalf("bounded recovery failed: %+v %v", r, err)
	}
	value, err := s.searchPage.Context(ctx).Eval(`()=>({search:searchClicks,filter:filterClicks,selected:document.querySelector('[data-index2="1"]').classList.contains('sDNqBVWH')})`)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Search, Filter int
		Selected       bool
	}
	if err := value.Value.Unmarshal(&state); err != nil || state.Search != 2 || state.Filter != 2 || !state.Selected {
		t.Fatalf("recovery lost filters or clicked too many times: %+v %v", state, err)
	}
}

func TestWebBrowserSearchReusesPageAcrossQueriesAndFilters(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>window.searchClicks=0;
const button=document.querySelector('[data-e2e="searchbar-button"]'),base=button.onclick;
button.onclick=function(){searchClicks++;base.call(this);filter(document.querySelector('[data-index2="0"]'));};
</script>`
	for i, r := range []*SearchRequest{
		{Query: "合成关键词甲"},
		{Query: "合成关键词乙"},
		{Query: "合成关键词乙", Filters: []FilterChoice{{Group: "排序依据", Option: "最新发布"}}},
	} {
		result, err := s.SearchPosts(ctx, r)
		if err != nil || len(result.Posts) != 1 {
			t.Fatalf("search %d failed: %+v %v", i, result, err)
		}
		value, err := s.searchPage.Context(ctx).Eval(`()=>window.searchClicks`)
		if err != nil || value.Value.Int() != i+1 {
			t.Fatalf("page was recreated or not submitted: %v %v", value, err)
		}
	}
}

func TestWebBrowserFilterStopsOnDelayedServerError(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>
window.filterClicks=0;
const result=document.querySelector('#search-result-container');
document.querySelector('[data-index2="1"]').onclick=function(){
 filterClicks++;filter(this);result.setAttribute('aria-busy','true');
 setTimeout(()=>{result.removeAttribute('aria-busy');result.innerHTML='<div>服务出现异常</div>';},500);
};
</script>`
	p, err := s.openSearch(ctx, &SearchRequest{Query: "合成分步筛选"})
	if err != nil {
		t.Fatal(err)
	}
	err = applyFilters(ctx, p, []FilterChoice{{Group: "排序依据", Option: "最新发布"}, {Group: "排序依据", Option: "综合排序"}})
	if !isSearchServerError(err) || !strings.Contains(err.Error(), "排序依据：最新发布") {
		t.Fatalf("missing exact failed filter: %v", err)
	}
	value, e := p.Eval(`()=>({clicks:filterClicks,selected:document.querySelector('[data-index2="1"]').classList.contains('sDNqBVWH')})`)
	if e != nil {
		t.Fatal(e)
	}
	var state struct {
		Clicks   int
		Selected bool
	}
	if e := value.Value.Unmarshal(&state); e != nil || state.Clicks != 1 || !state.Selected {
		t.Fatalf("continued filtering after failure: %+v %v", state, e)
	}
}

func TestWebBrowserFilterReacquiresElementReplacedOnHover(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>
window.filterClicks=0;
const latest=document.querySelector('[data-index2="1"]');
latest.setAttribute('onclick','filterClicks++;filter(this)');
latest.onmouseenter=function(){const fresh=this.cloneNode(true);this.replaceWith(fresh);};
</script>`
	r, err := s.SearchPosts(ctx, &SearchRequest{Query: "合成菜单替换", Filters: []FilterChoice{{Group: "排序依据", Option: "最新发布"}}})
	if err != nil || r == nil || len(r.Posts) != 1 {
		t.Fatalf("filter replacement did not recover: %+v %v", r, err)
	}
	value, err := s.searchPage.Context(ctx).Eval(`()=>filterClicks`)
	if err != nil || value.Value.Int() != 1 {
		t.Fatalf("filter was submitted more than once: %v %v", value, err)
	}
}

func TestWebBrowserSearchDetectsResetFilters(t *testing.T) {
	s, _, ctx := webFixture(t)
	p, err := s.openSearch(ctx, &SearchRequest{Query: "筛选完整性测试"})
	if err != nil {
		t.Fatal(err)
	}
	choices := []FilterChoice{{Group: "排序依据", Option: "最新发布"}}
	if err := applyFilters(ctx, p, choices); err != nil {
		t.Fatal(err)
	}
	if err := verifySearchFilters(ctx, p, choices); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Eval(`()=>filter(document.querySelector('[data-index2="0"]'))`); err != nil {
		t.Fatal(err)
	}
	if err := verifySearchFilters(ctx, p, choices); err == nil {
		t.Fatal("reset filter was reported as applied")
	}
}

func TestWebBrowserJingxuanFilterSelection(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html = strings.ReplaceAll(br.html, "sDNqBVWH", "HjptjtzN")
	br.html = strings.ReplaceAll(br.html, "'/search/'", "'/jingxuan/search/'")
	r, err := s.SearchPosts(ctx, &SearchRequest{Query: "合成图文", Filters: []FilterChoice{{Group: "排序依据", Option: "最新发布"}}})
	if err != nil || r == nil || len(r.Posts) != 1 {
		t.Fatalf("jingxuan filter state not verified: %+v %v", r, err)
	}
}

func TestWebBrowserActionsAreSingleAttemptAndPersistent(t *testing.T) {
	for _, kind := range []string{"like", "favorite", "comment", "reply", "comment_like"} {
		t.Run(kind, func(t *testing.T) {
			s, br, ctx := webFixture(t)
			yes := true
			r := &InteractionRequest{Post: "7665228646013364602"}
			switch kind {
			case "like":
				r.Liked = &yes
			case "favorite":
				r.Favorited = &yes
			case "comment":
				r.Text = "不错"
			case "reply", "comment_like":
				comments, err := s.GetComments(ctx, &CommentsRequest{Post: r.Post})
				if err != nil {
					t.Fatal(err)
				}
				children, err := s.GetComments(ctx, &CommentsRequest{Post: r.Post, ParentRef: comments.Comments[0].Ref})
				if err != nil {
					t.Fatal(err)
				}
				r.CommentRef = children.Comments[0].Ref
				if kind == "reply" {
					r.Text = "不错"
				} else {
					r.Liked = &yes
				}
			}
			preview, err := s.Interact(ctx, kind, r)
			if err != nil {
				t.Fatal(err)
			}
			p := s.active.page.Context(ctx)
			if countSubmissions(t, p) != 0 {
				t.Fatal("prepare mutated public state")
			}
			result, err := s.Interact(ctx, kind, &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Success || result.Stage != "completed" {
				t.Fatalf("unexpected result: %+v", result)
			}
			fresh := NewWebService(br, s.login, s.mediaRoot, s.dataDir)
			result, err = fresh.Interact(ctx, kind, &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
			if err != nil || !result.Success {
				t.Fatal("receipt not restored", err)
			}
			if countSubmissions(t, p) != 1 {
				t.Fatal("duplicate click after restart")
			}
		})
	}
}

func TestWebBrowserChangedEditorAndUnknownNeverResend(t *testing.T) {
	for _, kind := range []string{"changed", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			s, br, ctx := webFixture(t)
			preview, err := s.Interact(ctx, "comment", &InteractionRequest{Post: "7665228646013364602", Text: "不错"})
			if err != nil {
				t.Fatal(err)
			}
			p := s.active.page.Context(ctx)
			if kind == "changed" {
				_, err = p.Eval(`()=>document.querySelector('[data-dy-mcp-composer] [contenteditable]').textContent='changed'`)
			} else {
				_, err = p.Eval(`()=>window.showSuccess=false`)
			}
			if err != nil {
				t.Fatal(err)
			}
			limited, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			result, err := s.Interact(limited, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
			if err == nil {
				t.Fatal("invalid success")
			}
			if kind == "changed" {
				if countSubmissions(t, p) != 0 {
					t.Fatal("changed editor submitted")
				}
			} else {
				if result == nil || result.Stage != "unknown" {
					t.Fatalf("wanted unknown: %+v", result)
				}
				fresh := NewWebService(br, s.login, s.mediaRoot, s.dataDir)
				_, _ = fresh.Interact(ctx, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
				if countSubmissions(t, p) != 1 {
					t.Fatal("unknown send retried")
				}
			}
		})
	}
}

func TestWebBrowserImageMentionAndEmojiPreparation(t *testing.T) {
	s, _, ctx := webFixture(t)
	post := &PostRequest{Post: "7665228646013364602"}
	mentions, err := s.GetMentionCandidates(ctx, &MentionRequest{Post: post.Post, Query: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	emojis, err := s.GetEmojiOptions(ctx, post)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.Interact(ctx, "comment", &InteractionRequest{Post: post.Post, Text: "不错", ImagePath: "photo.png", Mentions: []string{mentions.Candidates[0].Ref}, Emojis: []string{emojis.Options[0].Ref}})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ImageName != "photo.png" || len(preview.Mentions) != 1 || preview.Stage != "ready" || countSubmissions(t, s.active.page.Context(ctx)) != 0 {
		t.Fatalf("bad preparation: %+v", preview)
	}
}

func TestWebBrowserStaticReactionTooltips(t *testing.T) {
	for _, kind := range []string{"like", "favorite"} {
		t.Run(kind, func(t *testing.T) {
			s, _, ctx := webFixture(t)
			p, _, err := s.openPost(ctx, "7665228646013364602")
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.Eval(`()=>{window.tip=kind=>{const e=document.getElementById(kind+'-tip');e.hidden=false;e.textContent=kind==='like'?'点赞':'收藏';};window.react=kind=>{submissions++;reactions[kind]=!reactions[kind];const i=kind==='like'?0:2,cell=document.querySelectorAll('.NoBOOMd6>.o2tLobnl')[i],icon=cell.querySelector('.s7K4YLGp');cell.classList.toggle('YY2jg5f8',reactions[kind]);icon.classList.toggle('prpiPAWb',!reactions[kind]);icon.querySelector('path').setAttribute('fill',reactions[kind]?(kind==='like'?'rgb(254,44,85)':'rgb(255,184,2)'):'rgb(255,255,255)');tip(kind);ackReaction(kind,reactions[kind]);};for(const i of [0,2])document.querySelectorAll('.NoBOOMd6>.o2tLobnl')[i].querySelector('[tabindex]').innerHTML='<div class="s7K4YLGp prpiPAWb"><svg width="30" height="30"><path d="M0 0L25 0L25 25Z" fill="rgb(255,255,255)" fill-opacity="1"/></svg></div>';}`)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []bool{true, false} {
				r := &InteractionRequest{Post: "7665228646013364602"}
				if kind == "like" {
					r.Liked = &want
				} else {
					r.Favorited = &want
				}
				preview, err := s.Interact(ctx, kind, r)
				if err != nil {
					t.Fatal(err)
				}
				result, err := s.Interact(ctx, kind, &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
				if err != nil || !result.Success {
					t.Fatal("static tooltip corrupted state", err)
				}
			}
			if countSubmissions(t, p) != 2 {
				t.Fatal("wrong reaction count")
			}
		})
	}
}

func TestWebBrowserFoldedDislikeAndUndo(t *testing.T) {
	s, _, ctx := webFixture(t)
	comments, err := s.GetComments(ctx, &CommentsRequest{Post: "7665228646013364602"})
	if err != nil {
		t.Fatal(err)
	}
	p := s.detailPage.Context(ctx)
	_, err = p.Eval(`()=>{
 const c=document.querySelector('[data-e2e="comment-item"]');
 c.querySelector('.HJa8wFwW').onclick=function(){
  submissions++;const folded=c.querySelector('.hv6PDgqY');
  if(folded){
   c.querySelector('.comment-item-stats-container').append(this);this.classList.remove('tOgCrAK_');folded.remove();
   const body=c.querySelector('.saved-body');body.className='FduGc_lz';body.hidden=false;
  }else{
   c.querySelector('.FduGc_lz').hidden=true;const panel=document.createElement('div');panel.className='hv6PDgqY';
   panel.innerHTML='<div class="WXRzm8gL">该评论被折叠</div>';this.classList.add('tOgCrAK_');panel.append(this);c.append(panel);c.querySelector('.FduGc_lz').className='saved-body';
  }
 };
}`)
	if err != nil {
		t.Fatal(err)
	}
	// Keep a detached original body, as the actual platform replaces it.
	for _, want := range []bool{true, false} {
		r := &InteractionRequest{Post: "7665228646013364602", CommentRef: comments.Comments[0].Ref, Disliked: &want}
		preview, err := s.Interact(ctx, "comment_dislike", r)
		if err != nil {
			t.Fatal(err)
		}
		result, err := s.Interact(ctx, "comment_dislike", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
		if err != nil || !result.Success {
			t.Fatal("dislike not confirmed", err)
		}
		comments, err = s.GetComments(ctx, &CommentsRequest{Post: r.Post})
		if err != nil {
			t.Fatal(err)
		}
	}
	if countSubmissions(t, p) != 2 {
		t.Fatal("wrong dislike count")
	}
}

func TestWebBrowserTargetAccountAndChallengeGuards(t *testing.T) {
	for _, change := range []string{"caption", "account", "target", "challenge"} {
		t.Run(change, func(t *testing.T) {
			s, _, ctx := webFixture(t)
			comments, err := s.GetComments(ctx, &CommentsRequest{Post: "7665228646013364602"})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := s.Interact(ctx, "reply", &InteractionRequest{Post: "7665228646013364602", CommentRef: comments.Comments[0].Ref, Text: "不错"})
			if err != nil {
				t.Fatal(err)
			}
			p := s.active.page.Context(ctx)
			_, err = p.Eval(`change=>{if(change==='caption')document.querySelector('h1').textContent='changed';if(change==='account')document.querySelector('#comment-input-container a').href='/user/someone-else';if(change==='target')document.querySelector('[data-e2e="comment-item"] .FduGc_lz').textContent='changed';if(change==='challenge'){const panel=document.createElement('div');panel.id='uc-second-verify';panel.innerHTML='<div class="second-verify-panel">请验证</div>';document.body.append(panel);}}`, change)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Interact(ctx, "reply", &InteractionRequest{ActionID: preview.ActionID, Confirm: true}); err == nil {
				t.Fatal("changed target accepted")
			}
			if countSubmissions(t, p) != 0 {
				t.Fatal("unsafe submission")
			}
		})
	}
}

func TestWebBrowserNoteLayoutAndTargetBinding(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>
const old=document.querySelector('[data-e2e="video-detail"]'),note=document.createElement('main');note.setAttribute('data-e2e','note-detail');while(old.firstChild)note.append(old.firstChild);old.replaceWith(note);
const info=note.querySelector('[data-e2e="detail-video-info"]');info.removeAttribute('data-e2e');info.innerHTML='<div data-e2e="user-info"><a href="https://www.douyin.com/user/note-author"><img alt="图文作者"></a></div><div><div class="Bfj9rfeR">合成图文作品</div><div class="RZ5JZlz_"><span class="mbFdUIBS">发布时间：今天</span></div></div>';
const feed=document.createElement('div');feed.setAttribute('data-e2e','feed-active-video');feed.setAttribute('data-e2e-vid','7665228646013364602');note.append(feed);
const list=note.querySelector('[data-e2e="comment-list"]');list.hidden=true;
const noteComposer=note.querySelector('#comment-input-container');const clone=noteComposer.querySelector('.comment-input-inner-container').cloneNode(true);noteComposer.hidden=true;note.append(clone);
for(const kind of ['digg','collect']){const button=document.createElement('div');button.setAttribute('data-e2e','video-player-'+kind);button.setAttribute('data-e2e-state',kind==='digg'?'video-player-no-digged':'video-player-no-collect');button.textContent='123';button.onclick=()=>{submissions++;const yes=button.getAttribute('data-e2e-state').includes('-no-');button.setAttribute('data-e2e-state',kind==='digg'?(yes?'video-player-is-digged':'video-player-no-digged'):(yes?'video-player-is-collected':'video-player-no-collect'));ackReaction(kind==='digg'?'like':'favorite',yes);};note.prepend(button);}
const toggle=document.createElement('div');toggle.setAttribute('data-e2e','feed-comment-icon');toggle.textContent='评论';toggle.onclick=()=>{list.hidden=false;};note.prepend(toggle);
</script>`
	post := "https://www.douyin.com/note/7665228646013364602"
	detail, err := s.GetPostDetail(ctx, &PostRequest{Post: post})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Kind != "image" || detail.Author != "图文作者" || detail.Description != "合成图文作品" || detail.Liked == nil || *detail.Liked {
		t.Fatalf("incorrect note detail: %+v", detail)
	}
	previewComment, err := s.Interact(ctx, "comment", &InteractionRequest{Post: post, Text: "不错"})
	if err != nil {
		t.Fatal("id-less note composer", err)
	}
	if previewComment.Stage != "ready" {
		t.Fatal("note comment not prepared")
	}
	comments, err := s.GetComments(ctx, &CommentsRequest{Post: post})
	if err != nil || len(comments.Comments) != 2 {
		t.Fatal("note comments", err)
	}
	yes := true
	preview, err := s.Interact(ctx, "like", &InteractionRequest{Post: post, Liked: &yes})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Interact(ctx, "like", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err != nil || !result.Success {
		t.Fatal("note reaction", err)
	}
	_, err = s.detailPage.Context(ctx).Eval(`()=>document.querySelector('[data-e2e="feed-active-video"]').setAttribute('data-e2e-vid','99999999999999')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPostDetail(ctx, &PostRequest{Post: post}); err == nil {
		t.Fatal("different note accepted")
	}
}
