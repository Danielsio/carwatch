import { ApiError } from "./api";

const knownMessages: Record<string, string> = {
  "search limit reached": "הגעת למגבלת החיפושים — מחק חיפוש קיים כדי ליצור חדש",
  "search name already exists": "שם החיפוש כבר קיים — בחר שם אחר",
  "search not found": "החיפוש לא נמצא",
  "listing not found": "המודעה לא נמצאה",
  "invalid JSON": "בקשה לא תקינה",
  "invalid JSON body": "בקשה לא תקינה",
  "invalid request body": "בקשה לא תקינה",
  "invalid search id": "מזהה חיפוש לא תקין",
  "invalid source": "מקור לא תקין",
  "invalid manufacturer id": "מזהה יצרן לא תקין",
  "missing token": "מזהה מודעה חסר",
  "token is required": "מזהה מודעה חסר",
  "endpoint is required": "כתובת התראה חסרה",
  "endpoint, keys.p256dh, and keys.auth are required": "פרטי מנוי התראות חסרים",
  "chat_id is required in body": "מזהה משתמש חסר",
  "valid chat_id is required": "מזהה משתמש לא תקין",
  "valid search id is required": "מזהה חיפוש לא תקין",
  "table is required": "שם טבלה חסר",
  "manufacturer is required for instant search": "יש לבחור יצרן לחיפוש מהיר",
  "saved listings limit reached": "הגעת למגבלת המודעות השמורות",
  "hidden listings limit reached": "הגעת למגבלת המודעות המוסתרות",
  "vacuum already in progress": "תחזוקת מסד הנתונים כבר רצה",
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
    return "שגיאה בלתי צפויה";
  }

  if (
    error instanceof TypeError &&
    /failed to fetch|network|load failed/i.test(error.message)
  ) {
    return "אין חיבור לשרת — בדוק את הרשת";
  }

  return "שגיאה בלתי צפויה";
}
