package douyin

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/liaogx/douyin-mcp/browser"
)

func postReactionButton(p *rod.Page, id, kind string) (*rod.Element, error) {
	if err := verifyPost(p, id); err != nil {
		return nil, err
	}
	index := 0
	if kind == "favorite" {
		index = 2
	} else if kind != "like" {
		return nil, problem("invalid_action", "未知作品互动", 400)
	}
	items, err := p.ElementsByJS(rod.Eval(`(id,index)=>{`+webDOMHelpers+postDOM+`
 const note=noteRoot(id);if(note)return all(index===0?'[data-e2e="video-player-digg"]':'[data-e2e="video-player-collect"]',note);
 const info=postInfo(id);if(!info)return [];
 const cells=all('.NoBOOMd6 > .o2tLobnl',info);
 if(cells.length!==4||cells[3].getAttribute('data-e2e')!=='video-share-icon-container')return [];
 return all('[tabindex="0"][aria-describedby]',cells[index]);
}`, id, index))
	return exactlyOne(items, err, "作品点赞/收藏按钮")
}

func postReactionState(p *rod.Page, id, kind string) (*bool, error) {
	el, err := postReactionButton(p, id, kind)
	if err != nil {
		return nil, err
	}
	semantic, err := el.Eval(`()=>this.getAttribute('data-e2e-state')||''`)
	if err != nil {
		return nil, err
	}
	if label := semantic.Value.Str(); label != "" {
		off, on := "video-player-no-digged", "video-player-is-digged"
		if kind == "favorite" {
			off, on = "video-player-no-collect", "video-player-is-collected"
		}
		if label == off {
			value := false
			return &value, nil
		}
		if label == on {
			value := true
			return &value, nil
		}
		return nil, problem("reaction_state_unknown", "作品互动状态标识已变化，未盲目切换", 409)
	}
	if err := browser.Hover(el); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(p.GetContext(), 4*time.Second)
	defer cancel()
	var state *bool
	err = poll(ctx, 180*time.Millisecond, func() (bool, error) {
		// The live site's tooltip says “点赞” even when already liked. Bind
		// the observed selected/off classes to the actually rendered SVG,
		// ignoring Lottie's hidden animation frames. Never infer false merely
		// from a missing selected class.
		visual, err := el.Context(ctx).Eval(`kind=>{
 const icon=this.querySelector('.s7K4YLGp'),cell=this.closest('.o2tLobnl');
 if(!icon||!cell)return null;
 const paths=[...icon.querySelectorAll('svg path')].filter(p=>{const r=p.getBoundingClientRect();return r.width>3&&r.height>3&&p.getAttribute('fill-opacity')==='1';});
 const fills=paths.map(p=>(p.getAttribute('fill')||'').replace(/\s/g,''));
 if(kind==='like'||kind==='favorite'){
  const activeFill=kind==='like'?'rgb(254,44,85)':'rgb(255,184,2)';
  if(cell.classList.contains('YY2jg5f8')&&!icon.classList.contains('prpiPAWb')&&fills.includes(activeFill))return true;
  if(!cell.classList.contains('YY2jg5f8')&&icon.classList.contains('prpiPAWb')&&fills.length>0&&fills.every(f=>f==='rgb(255,255,255)'))return false;
 }
 return null;
}`, kind)
		if err != nil {
			return false, err
		}
		var observed *bool
		if err := visual.Value.Unmarshal(&observed); err != nil {
			return false, err
		}
		if observed != nil {
			state = observed
			return true, nil
		}
		r, err := el.Context(ctx).Eval(`()=>{const id=this.getAttribute('aria-describedby');const tip=id&&document.getElementById(id);return tip&&tip.getClientRects().length?(tip.innerText||'').trim():'';}`)
		if err != nil {
			return false, err
		}
		off, on := "点赞", "取消点赞"
		if kind == "favorite" {
			off, on = "收藏", "取消收藏"
		}
		// Only use textual state on layouts without the known static-tooltip
		// icon. An animation or changed SVG must not fall back to “off”.
		known, err := el.Context(ctx).Eval(`()=>!!this.querySelector('.s7K4YLGp')`)
		if err != nil {
			return false, err
		}
		label := r.Value.Str()
		if label == off && !known.Value.Bool() {
			v := false
			state = &v
		}
		if label == on {
			v := true
			state = &v
		}
		return state != nil, nil
	})
	if err != nil {
		return nil, wrapTimeout(err, "无法从作品按钮的提示确认点赞/收藏状态，未盲目切换")
	}
	return state, nil
}

func commentReactionButton(p *rod.Page, id string, t commentTarget, kind string) (*rod.Element, error) {
	el, err := findComment(p, id, t)
	if err != nil {
		return nil, err
	}
	selector := ".comment-item-stats-container .VpA2NKl1,.hv6PDgqY .VpA2NKl1"
	if kind == "comment_dislike" {
		selector = ".comment-item-stats-container .HJa8wFwW,.hv6PDgqY .HJa8wFwW"
	} else if kind != "comment_like" {
		return nil, problem("invalid_action", "未知评论互动", 400)
	}
	items, err := el.ElementsByJS(rod.Eval(`selector=>{`+webDOMHelpers+commentDOM+`return own(this,selector).filter(visible);}`, selector))
	return exactlyOne(items, err, "评论点赞/点踩按钮")
}

func commentReactionState(p *rod.Page, id string, t commentTarget, kind string) (*bool, error) {
	el, err := commentReactionButton(p, id, t, kind)
	if err != nil {
		return nil, err
	}
	r, err := el.Eval(`kind=>{
 const aria=this.getAttribute('aria-pressed');if(aria==='true'||aria==='false')return aria==='true';
 const paths=[...this.querySelectorAll('svg path')];if(paths.length!==1)return null;
 const fill=(paths[0].getAttribute('fill')||'').toLowerCase();
 if(kind==='comment_like'){
  if(this.classList.contains('tOgCrAK_')&&fill==='#fe2c55')return true;
  if(!this.classList.contains('tOgCrAK_')&&fill==='#fff'&&paths[0].getAttribute('fill-opacity')==='.7')return false;
 }
 if(kind==='comment_dislike'){
  if(this.classList.contains('tOgCrAK_')&&this.closest('.hv6PDgqY')?.querySelector('.WXRzm8gL')?.innerText.includes('该评论被折叠'))return true;
  if(!this.classList.contains('tOgCrAK_')&&this.closest('.comment-item-stats-container')&&fill==='#fff'&&paths[0].getAttribute('fill-opacity')==='.5')return false;
 }
 return null;
}`, kind)
	if err != nil {
		return nil, err
	}
	if r.Value.Nil() {
		return nil, problem("reaction_state_unknown", "无法可靠判断评论点赞/点踩状态，未盲目切换", 409)
	}
	b := r.Value.Bool()
	return &b, nil
}
