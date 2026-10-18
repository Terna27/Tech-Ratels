import React from "react";

export function Spinner({ size = "md", text }: { size?: "sm" | "md" | "lg"; text?: string }) {
  const sizeClasses = {
    sm: "w-4 h-4 border-2",
    md: "w-6 h-6 border-2",
    lg: "w-10 h-10 border-3",
  };

  return (
    <div className="flex flex-col items-center justify-center py-8 gap-3">
      <div
        className={`${sizeClasses[size]} rounded-full animate-spin border-emerald-800 border-t-transparent`}
      />
      {text && <span className="text-sm text-gray-500 font-medium">{text}</span>}
    </div>
  );
}
