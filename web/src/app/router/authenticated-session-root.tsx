import { Outlet } from "react-router-dom";

import { ChannelAuthorizationPresenter } from "@/features/capability/channels/authorization/channel-authorization-presenter";

export function AuthenticatedAppSessionRoot() {
  return (
    <>
      <Outlet />
      <ChannelAuthorizationPresenter />
    </>
  );
}
