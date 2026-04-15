"use client";

import { useEffect, useState } from "react";

export function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(false);

  useEffect(() => {
    const mediaQuery = window.matchMedia(query);
    const update_matches = () => {
      setMatches(mediaQuery.matches);
    };

    update_matches();
    mediaQuery.addEventListener("change", update_matches);

    return () => {
      mediaQuery.removeEventListener("change", update_matches);
    };
  }, [query]);

  return matches;
}
