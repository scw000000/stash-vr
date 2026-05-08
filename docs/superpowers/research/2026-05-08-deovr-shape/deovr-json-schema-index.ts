export enum ScreenType {
  Flat = "flat",
  Dome = "dome",
  Sphere = "sphere",
  Fisheye = "fisheye",
  MKX200 = "mkx200",
  RF25 = "rf52",
}
export enum StereoMode {
  SideBySide = "sbs",
  TopBottom = "tb",
  OverUnder = "OverUnder",
  Mono = "mono",
}

export type VideoSource = {
  /** vertical resolution of the video */
  resolution: number;
  /** URL to the video file */
  url: string;
  /** vertical resolution of the video */
  height?: number;
  /** horizontal resolution of the video */
  width?: number;
  size?: {
    /** horizontal resolution of the video */
    w?: number;
    /** vertical resolution of the video */
    h?: number;
    /** size of the video file, in bytes? */
    size?: number;
  };
};
export type VideoEncoding = {
  name: "h264" | "h265" | string;
  videoSources: VideoSource[];
};
export type Corrections = {
  x?: number;
  y?: number;
  br?: number;
  cont?: number;
  sat?: number;
};
export type TimeStamp = {
  ts: number;
  name: string;
};

type BaseItem = {
  title?: string;
  id: number;
  /** URL to thumbnail image */
  thumbnailUrl?: string;
};
type ScreenMetaData = {
  id3d?: boolean;
  /**
   * "flat" - flat 2d video
   * "dome" - 180 degrees equirect mesh
   * "sphere" - 360 degrees equirect mesh
   * "fisheye" - 180 degrees fisheye mesh
   * "mkx200" - 200 degrees fisheye mesh
   * "rf52" - 190 degrees Canon fisheye mesh
   */
  screenType?: ScreenType;
  stereoMode?: StereoMode;
  viewAngle?: 180 | 360;
};
export type VideoLink = {
  title: string;
  /** URL to thumbnail image */
  thumbnailUrl: string;
  /** length of video in seconds */
  videoLength?: number;
  /** an absolute URL to the video json */
  video_url: string;
};
export type FullVideo = ScreenMetaData &
  BaseItem & {
    description?: string;
    encodings: VideoEncoding[];
    /** length of video in seconds */
    videoLength?: number;
    /**
     * URL to a video that is the same length as the original video
     * used to rewind and fast foward in the player
     * */
    videoThumbnail?: string;
    /** URL to a short video that will be used as a thumbnail */
    videoPreview?: string;
    /** List of time stamps */
    timeStamps?: TimeStamp[];
    /** the length of the intro in seconds */
    skipIntro?: number;
    fps?: number;
    /** unix time stamp */
    date?: number;
  };
export type Picture = BaseItem &
  ScreenMetaData & {
    /** URL to the image */
    path: string;
  };
export type Scene = {
  /** name of the scene */
  name: string;
  /** List of videos or pictures */
  list: (FullVideo | Picture | VideoLink)[];
};

export type AuthMetadata = {
  /**
   * Authorization in DeoVR
   * In case of transferring login, the result of server response should also contain field “authorized” with the following values:
   * 1 — user is successfully authorized;
   * 0 — user without an account;
   * -1 — authorization error.
   * In case of authorization error, DeoVR shows the following message: «Invalid login or password!».
   * If authorization is not used, the field is not required or its value should equal to 0.
   */
  authorized?: -1 | 0 | 1;
};
export type MultiVideoJson = AuthMetadata & {
  scenes?: Scene[];
};
export type SingleVideoJson = AuthMetadata & FullVideo;

export type DeoVrJson = (MultiVideoJson | SingleVideoJson) & {
  $schema: string;
};
