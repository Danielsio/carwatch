import { ApiError } from "./api";

const knownMessages: Record<string, string> = {
  "search limit reached": "הגעת למגבלת החיפושים — מחק חיפוש קיים כדי ליצור חדש",
  "search name already exists": "שם החיפוש כבר קיים — בחר שם אחר",
};

export function errorToHebrew(error: unknown): string {
  if (error instanceof ApiError) {
    const translated = knownMessages[error.message];
    if (translated) {
      return translated;
    }
    if (error.status === 401 || error.status === 403) {
      return "ההרשאה פגה — נסה להתחבר מחדש";
    }
    if (error.status === 409) {
      return error.message || "פעולה מתנגשת — נסה שוב";
    }
    if (error.status === 429) {
      return "יותר מדי בקשות — נסה שוב בעוד רגע";
    }
    if (error.status >= 500) {
      return "שגיאת שרת — נסה שוב מאוחר יותר";
    }
    return error.message || "שגיאה בלתי צפויה";
  }

  if (
    error instanceof TypeError &&
    /failed to fetch|network|load failed/i.test(error.message)
  ) {
    return "אין חיבור לשרת — בדוק את הרשת";
  }

  return "שגיאה בלתי צפויה";
}
