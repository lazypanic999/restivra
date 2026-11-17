import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
} from "react";
import { readLanguage, text, translateMessage } from "./i18n.js";
const Context = createContext(null);
export function LanguageProvider({ children }) {
  const [language, setValue] = useState(() => {
    try {
      return readLanguage(globalThis.localStorage);
    } catch {
      return "en";
    }
  });
  const t = useCallback(
    (key, values) => text(language, key, values),
    [language],
  );
  const setLanguage = (value) => {
    if (!["en", "tr"].includes(value)) return;
    setValue(value);
    try {
      localStorage.setItem("restivra.language", value);
    } catch {
      /* Preference remains usable without storage. */
    }
  };
  useEffect(() => {
    document.documentElement.lang = language;
    document.title = t("title");
  }, [language, t]);
  const explain = (message) => translateMessage(message, language);
  const formatDate = (value, full = false) => {
    if (!value || new Date(value).getFullYear() < 1900) return "—";
    return new Intl.DateTimeFormat(language === "tr" ? "tr-TR" : "en-GB", {
      day: "2-digit",
      month: "short",
      ...(full ? { year: "numeric" } : {}),
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(value));
  };
  return (
    <Context.Provider value={{ language, setLanguage, t, explain, formatDate }}>
      {children}
    </Context.Provider>
  );
}
export function useLanguage() {
  return useContext(Context);
}
