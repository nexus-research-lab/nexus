import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { ContactsPage } from "@/pages/contacts/contacts-page";
import { LauncherPage } from "@/pages/launcher/launcher-page";
import { NexusPage } from "@/pages/nexus/nexus-page";
import { RoomPage } from "@/pages/room/room-page";

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path={APP_ROUTE_PATHS.launcher} element={<LauncherPage />} />
        <Route path={APP_ROUTE_PATHS.nexus} element={<NexusPage />} />
        <Route path={APP_ROUTE_PATHS.nexus_conversation} element={<NexusPage />} />
        <Route path={APP_ROUTE_PATHS.room} element={<RoomPage />} />
        <Route path={APP_ROUTE_PATHS.room_conversation} element={<RoomPage />} />
        <Route path={APP_ROUTE_PATHS.contacts} element={<ContactsPage />} />
        <Route path={APP_ROUTE_PATHS.contact_profile} element={<ContactsPage />} />
        <Route path="*" element={<Navigate replace to={APP_ROUTE_PATHS.launcher} />} />
      </Routes>
    </BrowserRouter>
  );
}
