/* eslint-disable react-refresh/only-export-components -- compatibility shim */
import { toast as sonnerToast, Toaster } from "sonner";
import type { ReactNode } from "react";

export type ToastType = "success" | "error" | "info";

export interface ToastProviderProps {
  children: ReactNode;
}

export function ToastProvider({ children }: ToastProviderProps) {
  return (
    <>
      {children}
      <Toaster
        position="top-center"
        dir="rtl"
        toastOptions={{
          className: "font-sans",
        }}
        richColors
        closeButton
      />
    </>
  );
}

export interface ToastOptions {
  action?: {
    label: string;
    onClick: () => void;
  };
}

function compatToast(
  message: string,
  type?: ToastType,
  options?: ToastOptions,
) {
  const opts = options?.action
    ? {
        action: {
          label: options.action.label,
          onClick: options.action.onClick,
        },
      }
    : undefined;

  switch (type) {
    case "success":
      sonnerToast.success(message, opts);
      break;
    case "error":
      sonnerToast.error(message, opts);
      break;
    default:
      sonnerToast.info(message, opts);
      break;
  }
}

export function useToast() {
  return { toast: compatToast };
}

export function showGlobalToast(message: string, type?: ToastType) {
  compatToast(message, type);
}
