import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { ContactsPage } from "@/pages/contacts/contacts-page";
import { DmsPage } from "@/pages/dms/dms-page";
import { LauncherPage } from "@/pages/launcher/launcher-page";
import { RoomPage } from "@/pages/room/room-page";
import { RoomsPage } from "@/pages/rooms/rooms-page";

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path={APP_ROUTE_PATHS.launcher} element={<LauncherPage />} />
        <Route path={APP_ROUTE_PATHS.dm_directory} element={<DmsPage />} />
        <Route path={APP_ROUTE_PATHS.room_directory} element={<RoomsPage />} />
        <Route path={APP_ROUTE_PATHS.room} element={<RoomPage />} />
        <Route path={APP_ROUTE_PATHS.room_conversation} element={<RoomPage />} />
        <Route path={APP_ROUTE_PATHS.contacts} element={<ContactsPage />} />
        <Route path={APP_ROUTE_PATHS.contact_profile} element={<ContactsPage />} />
        <Route path="*" element={<Navigate replace to={APP_ROUTE_PATHS.launcher} />} />
      </Routes>
    </BrowserRouter>
  );
}
