// INPUT: 真实在线群页面和独立共享文件服务响应。
// OUTPUT: 无 Agent、无 Node 授权、无历史任务也能上传、幂等重试和下载共享文件。
// POS: 桌面与窄屏的共享工作区验收，不接触真实账号。
import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

test("shared files work before any local execution", async ({page,context},info) => {
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  const now="2026-09-16T00:00:00Z";
  const room={id:"files-room",name:"Files QA",organization_id:"org",membership_version:1,configuration_version:1,created_at:now,updated_at:now};
  const details={room,current_user_role:"owner",members:[],conversation:{id:"files-conversation",room_id:room.id,type:"main",high_water_message_seq:0,sync_stream_id:"files-stream",stream_epoch:"epoch",high_water_sync_event_seq:0}};
  const file={id:"shared-file",name:"report.txt",size:5,created_at:now,sha256:createHash("sha256").update("hello").digest("hex")};
  const commands:string[]=[];
  let uploaded=false;
  let sent: {content:{attachments?:typeof file[]}} | undefined;
  await context.routeWebSocket("**/nexus/v1/team/stream?*",()=>{});
  await context.route("**/*",async(route)=>{
    const request=route.request();const url=new URL(request.url());const path=url.pathname;
    const ok=(data:unknown)=>route.fulfill({json:{data}});
    if(path==="/nexus/v1/auth/status")return ok({...appShellRead("GET",path)!.data as object,auth_method:"password",control_user_id:"ui-fixture",organization_id:"org"});
    if(path==="/nexus/v1/team-node/room")return ok([]);
    if(path==="/nexus/v1/team-node")return ok({state:"disconnected",jobs:[],candidates:[],agent_ids:[]});
    if(path==="/nexus/v1/team/rooms")return ok({rooms:[details]});
    if(path==="/nexus/v1/team/invitations")return ok({invitations:[],recovery_rooms:[]});
    if(path===`/nexus/v1/team/rooms/${room.id}`)return ok(details);
    if(path.endsWith("/snapshot"))return ok({conversation_id:details.conversation.id,stream_id:"files-stream",stream_epoch:"epoch",snapshot_seq:0,messages:[],has_more:false,through_message_seq:0,next_message_seq:0});
    if(path===`/nexus/v1/team/conversations/${details.conversation.id}/messages`){
      sent=request.postDataJSON();
      return ok({message:{id:"attachment-message",conversation_id:details.conversation.id,message_seq:1,author_type:"user",author_user_id:"ui-fixture",author_username:"Me",author_display_name:"Me",client_message_id:request.headers()["idempotency-key"],content:sent!.content,created_at:now},high_water_seq:0});
    }
    if(path===`/nexus/v1/team/rooms/${room.id}/files`){
      if(request.method()==="GET")return ok(uploaded?[file]:[]);
      // WebKit 的拦截协议不提供 File 请求体；摘要仍验证浏览器读取了完整文件。
      if (info.project.use.browserName !== "webkit") expect(request.postDataBuffer()?.toString()).toBe("hello");
      expect(request.headers()["x-file-sha256"]).toBe(createHash("sha256").update("hello").digest("hex"));
      commands.push(request.headers()["idempotency-key"]);uploaded=true;
      return commands.length===1 ? route.fulfill({status:503,json:{message:"response unknown"}}) : ok(file);
    }
    if(path.endsWith("/files/shared-file"))return route.fulfill({body:"hello",contentType:"application/octet-stream"});
    const fixture=appShellRead(request.method(),path);if(fixture)return route.fulfill({json:fixture});
    if(["fetch","xhr"].includes(request.resourceType()))return ok([]);
    return route.continue();
  });
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/team?room_id=${room.id}&locale=zh&theme=light`)}`);
  await page.getByRole("button",{name:"工作区",exact:true}).click();
  await expect(page.getByText("群共享文件").first()).toBeVisible();
  await page.locator('input[type="file"][aria-label="上传文件"]').setInputFiles({name:"report.txt",mimeType:"text/plain",buffer:Buffer.from("hello")});
  await expect(page.getByText("上传未确认，请重试原文件。")).toBeVisible();
  await page.getByRole("button",{name:"重试",exact:true}).click();
  await expect(page.getByRole("button",{name:"report.txt",exact:true})).toBeVisible();
  expect(commands).toHaveLength(2);expect(commands[0]).toBe(commands[1]);
  const download=page.waitForEvent("download");
  await page.getByRole("button",{name:"report.txt",exact:true}).click();
  expect((await download).suggestedFilename()).toBe("report.txt");
  await page.screenshot({path:info.outputPath("shared-workspace.png")});
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/team?room_id=${room.id}&locale=zh&theme=light`)}`);
  const composer=page.getByPlaceholder("发送消息到群聊");
  await composer.fill("/");
  await expect(page.getByRole("option",{name:/browser/i})).toBeVisible();
  await composer.fill("");
  await page.locator('input[type="file"]').setInputFiles({name:"report.txt",mimeType:"text/plain",buffer:Buffer.from("hello")});
  await page.getByRole("button",{name:"发送消息",exact:true}).click();
  await expect.poll(()=>sent?.content.attachments?.[0]?.id).toBe(file.id);
  expect(sent?.content.attachments?.[0]).toEqual({id:file.id,name:file.name,size:file.size,sha256:file.sha256});
  await expect(page.getByRole("button",{name:/report.txt/})).toBeVisible();
  await page.screenshot({path:info.outputPath("message-attachment.png")});
});
