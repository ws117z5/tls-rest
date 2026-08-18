// Allow importing style assets in TypeScript (handled by webpack loaders).
declare module "*.css";
declare module "*.scss";
declare module "*.sass";

interface ImportMeta {
    webpackContext(
        request: string,
        options?: {
            recursive?: boolean;
            regExp?: RegExp;
            mode?: "sync" | "eager" | "weak" | "lazy" | "lazy-once";
        }
    ): {
        keys(): string[];
        <T = any>(id: string): T;
        resolve(id: string): string;
        id: string | number;
    };
}