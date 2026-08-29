import React from "react";
import { ModuleViewProps } from "@engine/controllers/registry";
import { imageUrl, ImageRef } from "./controllers/images";

// Custom LIST view for the images module: a responsive grid of image cards, each
// showing the picture with its filename and upload date. Admins can switch back
// to the standard table via the layout toggle in the page chrome.
const ImagesList: React.FC<ModuleViewProps> = ({ data, navigate, module }) => {
    const rows = Array.isArray(data) ? data : [];

    if (rows.length === 0) {
        return <div className="text-muted p-3">No images.</div>;
    }

    const fmtDate = (v: any): string => {
        if (!v) return "";
        const d = new Date(v);
        if (isNaN(d.getTime())) return String(v);
        return `${d.toLocaleDateString()} ${d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
    };

    return (
        <div
            style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
                gap: 16,
            }}
        >
            {rows.map((row: any) => {
                const ref: ImageRef = {
                    id: row.id,
                    uuid: row.uuid,
                    filename: row.filename,
                    mime_type: row.mime_type,
                };
                const name = row.filename || `#${row.id}`;
                return (
                    <div
                        key={row.id}
                        className="card h-100"
                        style={{ cursor: "pointer" }}
                        onClick={() => navigate(`/${module}/${row.id}`)}
                        title={name}
                    >
                        <div style={{ aspectRatio: "1 / 1", overflow: "hidden", background: "#f5f5f5" }}>
                            <img
                                src={imageUrl(ref)}
                                alt={name}
                                loading="lazy"
                                style={{ width: "100%", height: "100%", objectFit: "cover" }}
                            />
                        </div>
                        <div className="card-body p-2">
                            <div className="text-truncate small fw-semibold">{name}</div>
                            <div className="text-muted" style={{ fontSize: 12 }}>
                                {fmtDate(row.created)}
                            </div>
                        </div>
                    </div>
                );
            })}
        </div>
    );
};

export default ImagesList;