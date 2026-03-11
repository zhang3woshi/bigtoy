import{f as y}from"./api-BP5h_7wt.js";const c=document.getElementById("loading"),i=document.getElementById("error"),u=document.getElementById("empty"),g=document.getElementById("model-grid"),p=document.getElementById("search-input"),s=document.getElementById("brand-filter"),E=document.getElementById("total-count"),r={all:[],filtered:[]};function a(e){return String(e??"").replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/\"/g,"&quot;").replace(/'/g,"&#39;")}function $(e){return!Array.isArray(e)||e.length===0?"":`<div class="tags">${e.map(t=>`<span class="tag">${a(t)}</span>`).join("")}</div>`}function v(e){return e?.id?`/model.html?${new URLSearchParams({id:String(e.id)}).toString()}`:"/model.html"}function b(e){const t=e.brand||"Unknown",n=String(e.modelCode||"").trim(),o=e.scale||"-",l=e.condition||"-",d=e.year||"-",f=e.imageUrl?`<img src="${a(e.imageUrl)}" alt="${a(e.name)}" loading="lazy" />`:'<div class="cover-placeholder">No Image</div>';return`
    <a class="card-link" href="${v(e)}" aria-label="查看 ${a(e.name)} 详情">
      <article class="model-card">
        <div class="cover">${f}</div>
        <div class="card-body">
          <h3>${a(e.name)}</h3>
          <p class="model-code">编号 ${a(n||"未填写")}</p>
          <p class="sub">${a(t)} · ${a(e.series||"未分类")}</p>
          <p class="meta">年份 ${a(d)} · 比例 ${a(o)} · 品相 ${a(l)}</p>
          ${$(e.tags)}
          <p class="note">${a(e.notes||"暂无备注")}</p>
        </div>
      </article>
    </a>
  `}function m(){const e=p.value.trim().toLowerCase(),t=s.value;r.filtered=r.all.filter(n=>{const o=[n.name,n.modelCode,n.series,n.notes,...n.tags||[],...n.gallery||[]].join(" ").toLowerCase(),l=!e||o.includes(e),d=t==="all"||n.brand===t;return l&&d}),h()}function h(){if(E.textContent=String(r.all.length),r.filtered.length===0){u.classList.remove("hidden"),g.innerHTML="";return}u.classList.add("hidden"),g.innerHTML=r.filtered.map(b).join("")}function L(){[...new Set(r.all.map(t=>t.brand).filter(Boolean))].sort().forEach(t=>{const n=document.createElement("option");n.value=t,n.textContent=t,s.appendChild(n)})}async function C(){try{const e=await y();r.all=e,r.filtered=e,c.classList.add("hidden"),L(),h()}catch(e){c.classList.add("hidden"),i.classList.remove("hidden"),i.textContent=`加载失败：${e.message}`}}p.addEventListener("input",m);s.addEventListener("change",m);C();
