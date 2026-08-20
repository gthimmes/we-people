import { useEffect, useRef } from "react";
import { useLocation } from "react-router-dom";
import { HelpNavigator } from "help-navigator";
import { helpContent } from "../help/content";
import { helpArticlesFor } from "../help/context";

// Mounts the in-app help center (floating launcher bottom-right, F1 to
// toggle) and keeps "Suggested for this page" in sync with the route.
export default function HelpWidget() {
  const location = useLocation();
  const helpRef = useRef<HelpNavigator | null>(null);

  useEffect(() => {
    const help = HelpNavigator.init({
      content: helpContent,
      theme: "light",
      accentColor: "#4f46e5",
      position: "bottom-right",
      hotkey: "F1",
      texts: { panelTitle: "Help center" },
    });
    helpRef.current = help;
    return () => {
      helpRef.current = null;
      help.destroy();
    };
  }, []);

  useEffect(() => {
    helpRef.current?.setContext(helpArticlesFor(location.pathname));
  }, [location.pathname]);

  return null;
}
