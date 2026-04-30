/* eslint-disable react-refresh/only-export-components -- buttonVariants exported for composition */
import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-xl font-medium transition-all duration-200 active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary:
          "bg-primary text-primary-foreground shadow-[0_0_20px_rgba(59,130,246,0.25)] hover:opacity-90",
        secondary: "bg-secondary text-secondary-foreground hover:bg-accent",
        ghost: "bg-transparent text-foreground hover:bg-accent",
        destructive:
          "bg-destructive text-destructive-foreground hover:opacity-90",
      },
      size: {
        sm: "h-8 px-3 text-sm",
        md: "h-10 px-4 text-sm",
        lg: "h-12 px-6 text-base",
        icon: "h-10 w-10",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  },
);

type ButtonOwnProps = VariantProps<typeof buttonVariants> & {
  asChild?: boolean;
};

export type ButtonProps<T extends React.ElementType = "button"> =
  ButtonOwnProps & {
    as?: T;
  } & Omit<
    React.ComponentPropsWithoutRef<T>,
    keyof ButtonOwnProps | "as"
  >;

function ButtonRender(
  props: ButtonProps<React.ElementType>,
  ref: React.ComponentPropsWithRef<React.ElementType>["ref"],
): React.ReactElement {
  const {
    as,
    asChild = false,
    className,
    variant,
    size,
    type,
    children,
    ...rest
  } = props as ButtonProps<"button"> & { as?: React.ElementType };

  const classes = cn(buttonVariants({ variant, size }), className);

  if (asChild) {
    if (!React.isValidElement(children)) {
      throw new Error("כפתור עם asChild דורש ילד יחיד מסוג אלמנט");
    }
    const childProps = children.props as { className?: string };
    return React.cloneElement(
      children as React.ReactElement<Record<string, unknown>>,
      {
        ...rest,
        className: cn(classes, childProps.className),
        ref,
      },
    );
  }

  const Comp = (as ?? "button") as React.ElementType;
  return (
    <Comp
      ref={ref}
      className={classes}
      type={Comp === "button" ? (type ?? "button") : undefined}
      {...rest}
    >
      {children}
    </Comp>
  );
}

const ButtonBase = React.forwardRef(ButtonRender);

export const Button = ButtonBase as <
  T extends React.ElementType = "button",
>(
  props: ButtonProps<T> & {
    ref?: React.ComponentPropsWithRef<T>["ref"];
  },
) => React.ReactElement;

ButtonBase.displayName = "Button";

export { buttonVariants };
