import { expect, test, type WebSocketRoute } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

test("human contact opens a DM, sends durable messages and accepts a group invitation", async ({page,context},info) => {
 const now="2026-09-16T00:00:00Z";
 const zh=info.project.metadata.locale==="zh";
 const room={id:"dm",direct_user_id:"peer",organization_id:"org",name:"Direct message",description:"",avatar:"",host_auto_reply_enabled:false,private_messages_enabled:false,skill_names:[],configuration_version:1,membership_version:1,created_at:now,updated_at:now};
 const details={room,current_user_role:"member",members:[],conversation:{id:"dm-conversation",room_id:"dm",type:"main",high_water_message_seq:0,last_activity_at:null,sync_stream_id:"dm-stream",stream_epoch:"epoch",high_water_sync_event_seq:0}};
 const messages:Record<string,unknown>[]=[{id:"invitation",conversation_id:"dm-conversation",message_seq:1,author_type:"user",author_user_id:"peer",author_username:"peer",author_display_name:"Alice",client_message_id:"invite",content:{version:1,blocks:[{type:"room_invitation",text:"Research",room_id:"group",invitee_user_id:"ui-fixture",invited_at:now}]},created_at:now}];
 messages.push({id:"hello",conversation_id:"dm-conversation",message_seq:2,author_type:"user",author_user_id:"peer",author_username:"peer",author_display_name:"Alice",client_message_id:"hello",content:{version:1,blocks:[{type:"markdown",text:"Hello from Alice"}]},created_at:now});
 details.conversation.high_water_sync_event_seq=2;details.conversation.high_water_message_seq=2;
 let opened=false, accepted=false;
 const clearKeys:string[]=[];
 const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
 await context.addInitScript(APP_SHELL_INIT_SCRIPT);
 await context.routeWebSocket("**/nexus/v1/chat/ws",socket=>socket.onMessage(raw=>{if(JSON.parse(raw.toString()).type==="ping")socket.send(JSON.stringify({event_type:"pong"}));}));
 let messageStream: WebSocketRoute | undefined;
 await context.routeWebSocket("**/nexus/v1/team/stream?*",socket=>{messageStream=socket;});
 await context.route("**/*",async route=>{
  const req=route.request(),url=new URL(req.url()),path=url.pathname;
  const data=(value:unknown)=>route.fulfill({json:{data:value}});
  if(path==="/nexus/v1/auth/status")return data({...appShellRead("GET",path)!.data as object,auth_method:"password",control_user_id:"ui-fixture",organization_id:"org"});
  if(path==="/auth/v1/directory/members")return data([{user_id:"peer",username:"peer",display_name:"Alice",avatar:"3"}]);
  if(path==="/auth/v1/directory/agents" || path==="/auth/v1/agents")return data([]);
  if(path==="/nexus/v1/team/rooms") {
   if(req.method()==="POST") {expect(req.postDataJSON().direct_user_id).toBe("peer");expect(req.headers()["idempotency-key"]).toBeTruthy();opened=true;return data(details);}
   return data({rooms:opened?[details]:[]});
  }
  if(path==="/nexus/v1/team/rooms/dm") {
   if(req.method()==="PATCH") {
    expect(req.postDataJSON().hide_direct).toBe(true);
    const key=req.headers()["idempotency-key"];clearKeys.push(key);
    if(clearKeys.length===1) {opened=false;return route.abort();}
    expect(key).toBe(clearKeys[0]);return data({room_id:"dm",configuration_version:1,replayed:true});
   }
   return data(details);
  }
  if(path==="/nexus/v1/team/invitations")return data({invitations:opened&&!accepted?[{room:{...room,id:"group",direct_user_id:undefined,name:"Research"},invited_by_user_id:"peer",created_at:now}]:[],recovery_rooms:[]});
  if(path==="/nexus/v1/team/rooms/group/invitations/accept") {accepted=true;return data({room_id:"group",membership_version:2,replayed:false});}
  if(path.endsWith("/snapshot"))return data({conversation_id:"dm-conversation",stream_id:"dm-stream",stream_epoch:"epoch",snapshot_seq:messages.length,messages,has_more:false,through_message_seq:messages.length,next_message_seq:1});
  if(path.endsWith("/difference"))return data({next_seq:messages.length,high_water_seq:messages.length,events:messages.map((message,i)=>({event_seq:i+1,event_type:"message.created",message}))});
  if(path.endsWith("/messages") && req.method()==="POST") {
   const input=req.postDataJSON();messages.push({id:"message",conversation_id:"dm-conversation",message_seq:3,author_type:"user",author_user_id:"ui-fixture",author_username:"me",author_display_name:"Me",client_message_id:req.headers()["idempotency-key"],content:input.content,created_at:now});
   details.conversation.high_water_sync_event_seq=messages.length;details.conversation.high_water_message_seq=messages.length;
   return data({message:messages.at(-1),stream_id:"dm-stream",stream_epoch:"epoch",event_seq:messages.length,high_water_seq:messages.length,replayed:false});
  }
  const fixture=appShellRead(req.method(),path);if(fixture)return route.fulfill({json:fixture});
  if(!["localhost","127.0.0.1"].includes(url.hostname)||req.method()!=="GET")return route.abort();
  if(["fetch","xhr"].includes(req.resourceType()))return data([]);
  return route.continue();
 });
 await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/contacts?theme=${info.project.metadata.theme}&locale=${info.project.metadata.locale}`)}`);
 await expect(page.getByRole("button",{name:zh?"发起私聊":"New direct message",exact:true})).toHaveCount(0);
 await page.getByRole("button",{name:zh?"组织成员":"Organization members",exact:true}).click();
 // 窄视口走 app shell sidebar 形态（contacts-sidebar-panel → HumanContactsDirectory sidebar）：
 // 该分支没有 section/region，成员直接渲染为 “Alice @peer” 列表行，搜索框是 SidebarSearchField。
 const isNarrow=()=>page.viewportSize()!.width<768;
 const directory=isNarrow()?page:page.getByRole("region",{name:zh?"组织成员":"Organization members",exact:true});
 const memberText=directory.getByText("Alice",{exact:true});
 await expect(memberText).toBeVisible();
 const search=directory.getByRole("searchbox");
 await search.fill("no-match");
 await expect(memberText).toHaveCount(0);
 await search.fill("");
 await expect(memberText).toBeVisible();
 await directory.screenshot({path:info.outputPath("organization-member-directory.png")});
 const memberRow=page.getByRole("button",{name:"Alice @peer",exact:true});
 if (isNarrow()) {
  // sidebar 行点击是选中导航而非发起私聊；发起聊天动作在行尾 hover 按钮上。
  await memberRow.hover();
  await memberRow.getByRole("button").click();
 } else if (zh) {
  await memberRow.click();
  expect(opened).toBe(false);
  await page.getByRole("button",{name:"发消息: Alice",exact:true}).click();
 } else {
  await memberRow.hover();
  await memberRow.getByRole("button").click();
 }
 const input=page.getByPlaceholder(zh?"发送私聊消息":"Send a direct message");await expect(input).toBeVisible();
 await expect(page.getByRole("button",{name:zh?"本机授权":"Host authorization",exact:true})).toHaveCount(0);
 await expect(page.getByRole("button",{name:zh?"工作图":"Work graph",exact:true})).toHaveCount(0);
 await expect(page.getByText("Hello from Alice",{exact:true})).toBeVisible();
 await input.fill("Hello Alice");await input.press("Enter");
 await expect(page.getByText("Hello Alice",{exact:true})).toBeVisible();
 // 对方的新消息必须由推送补拉，不能依赖刷新页面或再次发送。
 await expect.poll(()=>Boolean(messageStream)).toBe(true);
 messages.push({...messages[1],id:"reply",message_seq:4,client_message_id:"reply",content:{version:1,blocks:[{type:"markdown",text:"Alice replies live"}]}});
 details.conversation.high_water_sync_event_seq=4;details.conversation.high_water_message_seq=4;
 messageStream!.send(JSON.stringify({type:"stream.updated",stream_id:"dm-stream",stream_epoch:"epoch",high_water_seq:4}));
 await expect(page.getByText("Alice replies live",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:zh?"加入":"Join",exact:true}).click();
 await expect.poll(()=>accepted).toBe(true);
 await page.reload();await expect(page.getByText("Hello Alice",{exact:true})).toBeVisible();
 expect(messages).toHaveLength(4);
 // “移出列表”入口只存在于 sidebar 聊面板（chat-sidebar-panel 的行级删除按钮），窄视口
 // shell 不渲染该面板；这里回到宽视口验证同一段 hide_direct 产品逻辑。
 if (isNarrow()) await page.setViewportSize({width:1440,height:900});
 const deleteButton=page.getByRole("button",{name:zh?"移出列表":"Remove from list",exact:true});
 await deleteButton.first().click({force:true});
 const dialog=page.getByRole("dialog");
 await expect(dialog).toContainText(zh?"聊天记录都会保留":"keep their history");
 await dialog.getByRole("button",{name:zh?"移出列表":"Remove from list",exact:true}).click();
 await expect(dialog).toContainText(zh?"暂时无法确认移出结果":"Removal could not be confirmed");
 await dialog.getByRole("button",{name:zh?"移出列表":"Remove from list",exact:true}).click();
 await expect(dialog).toHaveCount(0);
 await expect(input).toHaveCount(0);
 await page.goto("/app.html?desktop_route="+encodeURIComponent("/contacts?view=members"));
 const restoredRow=page.getByRole("button",{name:"Alice @peer",exact:true});
 if (isNarrow()) {
  await restoredRow.hover();
  await restoredRow.getByRole("button").click();
 } else {
  await restoredRow.click();
  await page.getByRole("button",{name:zh?"发消息: Alice":"Message: Alice",exact:true}).click();
 }
 await expect(input).toBeVisible();
 await expect(page.getByText("Hello from Alice",{exact:true})).toBeVisible();
 await expect(page.getByText("Hello Alice",{exact:true})).toBeVisible();
 await expect(page.getByText("Alice replies live",{exact:true})).toBeVisible();
 expect(messages).toHaveLength(4);expect(clearKeys).toHaveLength(2);expect(errors).toEqual([]);
 await page.screenshot({path:info.outputPath("human-dm.png")});
});
